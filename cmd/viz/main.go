// Command viz shows an execution of the lab in a web page: the throughput,
// errors and latency of each second, per node, the stale reads of the E3
// coordinator, the state of each node (leader, follower, primary, replica,
// down) and the faults injected, marked on the timeline.
//
// The page drives the real environment. It starts the cluster of either
// system with lab/scripts/up.sh, runs loadgen and stale inside the gerador
// container, and injects faults with the scripts of lab/scripts/ and docker
// kill/start. It must run on the host, from the project root:
//
//	go run ./cmd/viz            # serves http://localhost:8080
//	go run ./cmd/viz -addr :9000
//
// The rest of the project does not depend on this command.
package main

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

//go:embed index.html
var indexHTML []byte

func main() {
	addr := flag("addr", "localhost:8080")
	root, err := projectRoot()
	if err != nil {
		log.Fatal(err)
	}
	env, err := loadEnv(filepath.Join(root, "lab", ".env"))
	if err != nil {
		log.Fatal(err)
	}
	s := newServer(root, env)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("POST /api/cluster", s.handleCluster)
	mux.HandleFunc("POST /api/load", s.handleLoad)
	mux.HandleFunc("POST /api/stale", s.handleStale)
	mux.HandleFunc("POST /api/stop", s.handleStop)
	mux.HandleFunc("POST /api/fault", s.handleFault)

	log.Printf("visualization at http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func flag(name, def string) string {
	for i, a := range os.Args[1:] {
		if a == "-"+name && i+2 < len(os.Args) {
			return os.Args[i+2]
		}
		if v, ok := strings.CutPrefix(a, "-"+name+"="); ok {
			return v
		}
	}
	return def
}

// projectRoot finds the directory that holds lab/.env, starting from the
// working directory.
func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "lab", ".env")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("run from the project root (the directory with lab/.env)")
		}
		dir = parent
	}
}

func loadEnv(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	env := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok && !strings.HasPrefix(k, "#") {
			env[k] = v
		}
	}
	return env, nil
}

var nodeNames = []string{"n1", "n2", "n3"}

// maxHistory bounds the events kept for pages that connect later.
const maxHistory = 200_000

type server struct {
	root string
	env  map[string]string

	mu      sync.Mutex
	subs    map[chan []byte]struct{}
	history [][]byte
	start   time.Time // session start: when the cluster was started
	system  string    // "" when no cluster is up
	tool    *tool     // loadgen or stale in progress, if any
	busy    bool      // a cluster is being started or stopped
	stopNS  chan struct{}
}

func newServer(root string, env map[string]string) *server {
	s := &server{root: root, env: env, subs: map[chan []byte]struct{}{}}
	s.start = time.Now()
	go s.pollNodes()
	return s
}

func (s *server) clientIP(node string) string {
	return s.env["LAB_NET_CLIENTS"] + ".1" + strings.TrimPrefix(node, "n")
}

func (s *server) now() float64 { return time.Since(s.start).Seconds() }

// publish sends the event to every connected page and keeps it for later
// pages.
func (s *server) publish(ev map[string]any) {
	if _, ok := ev["t"]; !ok {
		ev["t"] = s.now()
	}
	b, err := json.Marshal(ev)
	if err != nil {
		log.Printf("event: %v", err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.history) < maxHistory {
		s.history = append(s.history, b)
	}
	for c := range s.subs {
		select {
		case c <- b:
		default:
			delete(s.subs, c)
			close(c)
		}
	}
}

func (s *server) logf(format string, args ...any) {
	s.publish(map[string]any{"type": "log", "text": fmt.Sprintf(format, args...)})
}

func (s *server) event(kind, label string) {
	s.publish(map[string]any{"type": "event", "kind": kind, "label": label})
}

func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	c := make(chan []byte, 4096)
	s.mu.Lock()
	history := s.history
	s.subs[c] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if _, ok := s.subs[c]; ok {
			delete(s.subs, c)
			close(c)
		}
		s.mu.Unlock()
	}()

	fmt.Fprint(w, ": connected\n\n") // some bytes at once, so the browser reports the stream open
	for _, b := range history {
		fmt.Fprintf(w, "data: %s\n\n", b)
	}
	flusher.Flush()
	for {
		select {
		case b, ok := <-c:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", b)
			for more := true; more; {
				select {
				case b, ok := <-c:
					if !ok {
						return
					}
					fmt.Fprintf(w, "data: %s\n\n", b)
				default:
					more = false
				}
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func decode(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(v)
}

func fail(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusBadRequest)
}

// handleCluster starts a clean cluster of the given system, or stops the
// current one with system "".
func (s *server) handleCluster(w http.ResponseWriter, r *http.Request) {
	var p struct{ System string }
	if err := decode(r, &p); err != nil {
		fail(w, err)
		return
	}
	if p.System != "" && p.System != "etcd" && p.System != "redis" {
		fail(w, errors.New("system must be etcd, redis or empty"))
		return
	}
	s.mu.Lock()
	if s.busy || s.tool != nil {
		s.mu.Unlock()
		fail(w, errors.New("a tool is running or the cluster is changing; stop it first"))
		return
	}
	s.busy = true
	s.mu.Unlock()
	w.WriteHeader(http.StatusAccepted)

	go func() {
		defer func() {
			s.mu.Lock()
			s.busy = false
			s.mu.Unlock()
		}()
		if p.System == "" {
			s.event("cluster", "cluster stopped")
			s.runScript("down.sh")
			s.mu.Lock()
			s.system = ""
			s.mu.Unlock()
			s.publish(map[string]any{"type": "session", "system": ""})
			return
		}
		s.mu.Lock()
		s.system = ""
		s.start = time.Now()
		s.history = nil
		s.mu.Unlock()
		s.publish(map[string]any{"type": "session", "system": p.System, "starting": true, "nodes": nodeNames})
		if err := s.runScript("up.sh", p.System); err != nil {
			s.logf("up.sh failed: %v", err)
			return
		}
		s.mu.Lock()
		s.system = p.System
		s.mu.Unlock()
		s.publish(map[string]any{"type": "session", "system": p.System, "nodes": nodeNames})
		s.event("cluster", p.System+" cluster ready")
	}()
}

// runScript runs lab/scripts/<name> and publishes its output.
func (s *server) runScript(name string, args ...string) error {
	cmd := exec.Command(filepath.Join(s.root, "lab", "scripts", name), args...)
	cmd.Dir = s.root
	out, err := cmd.CombinedOutput()
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" && !noise(line) {
			s.logf("%s: %s", name, line)
		}
	}
	return err
}

// noise reports the lines of docker compose and docker build output that
// the log leaves out.
func noise(line string) bool {
	for _, p := range []string{"Container ", "Network ", "Image ", "Step ", "--->", "Sending build", "Successfully ", "time=", "Removing intermediate", "Volume "} {
		if strings.HasPrefix(line, p) {
			return true
		}
	}
	return false
}

// tool is loadgen or stale running inside the gerador container.
type tool struct {
	name string
	cmd  *exec.Cmd
	done chan struct{}
}

type loadParams struct {
	Threads  int     `json:"threads"`
	Duration int     `json:"duration"` // seconds
	Warmup   int     `json:"warmup"`   // seconds
	PutRatio float64 `json:"putRatio"`
	Read     string  `json:"read"`
	Pinned   bool    `json:"pinned"`
}

func (s *server) handleLoad(w http.ResponseWriter, r *http.Request) {
	var p loadParams
	if err := decode(r, &p); err != nil {
		fail(w, err)
		return
	}
	if p.Threads < 1 || p.Threads > 4096 || p.Duration < 2 || p.Warmup < 0 || p.Warmup >= p.Duration {
		fail(w, errors.New("threads must be 1 to 4096, warm-up shorter than the duration"))
		return
	}
	args := []string{"loadgen", "-live", "-preload=true",
		"-threads", strconv.Itoa(p.Threads),
		"-duration", fmt.Sprintf("%ds", p.Duration),
		"-warmup", fmt.Sprintf("%ds", p.Warmup),
		"-put", strconv.FormatFloat(p.PutRatio, 'f', 2, 64),
	}
	if p.Read != "" {
		args = append(args, "-read", p.Read)
	}
	if p.Pinned {
		args = append(args, "-pinned")
	}
	label := fmt.Sprintf("load N=%d for %ds", p.Threads, p.Duration)
	if err := s.startTool("loadgen", label, args, func(line []byte) {
		var iv map[string]any
		if json.Unmarshal(line, &iv) == nil {
			iv["type"] = "interval"
			iv["t"] = s.at(iv["unix_ms"])
			s.publish(iv)
		}
	}); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type staleParams struct {
	Pairs     int    `json:"pairs"`
	WriteNode int    `json:"writeNode"`
	ReadNode  int    `json:"readNode"`
	Read      string `json:"read"`
}

func (s *server) handleStale(w http.ResponseWriter, r *http.Request) {
	var p staleParams
	if err := decode(r, &p); err != nil {
		fail(w, err)
		return
	}
	if p.Pairs < 1 || p.WriteNode < 1 || p.WriteNode > 3 || p.ReadNode < 1 || p.ReadNode > 3 {
		fail(w, errors.New("pairs must be positive and nodes between 1 and 3"))
		return
	}
	args := []string{"stale", "-live",
		"-pairs", strconv.Itoa(p.Pairs),
		"-write-node", strconv.Itoa(p.WriteNode),
		"-read-node", strconv.Itoa(p.ReadNode),
	}
	if p.Read != "" {
		args = append(args, "-read", p.Read)
	}
	label := fmt.Sprintf("stale reads: %d pairs, PUT n%d then GET n%d", p.Pairs, p.WriteNode, p.ReadNode)
	if err := s.startTool("stale", label, args, func(line []byte) {
		var iv map[string]any
		if json.Unmarshal(line, &iv) == nil {
			iv["type"] = "stale"
			iv["t"] = s.at(iv["unix_ms"])
			s.publish(iv)
		}
	}); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// at converts a unix_ms field of a tool event to session seconds.
func (s *server) at(unixMs any) float64 {
	ms, _ := unixMs.(float64)
	if ms == 0 {
		return s.now()
	}
	return float64(time.UnixMilli(int64(ms)).Sub(s.start)) / 1e9
}

// startTool runs a command inside the gerador container. Each line of its
// standard output goes to onLine; its standard error goes to the log.
func (s *server) startTool(name, label string, args []string, onLine func([]byte)) error {
	cmd := exec.Command("docker", append([]string{"exec", "gerador"}, args...)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	t := &tool{name: name, cmd: cmd, done: make(chan struct{})}

	s.mu.Lock()
	switch {
	case s.system == "":
		err = errors.New("no cluster is up")
	case s.tool != nil:
		err = fmt.Errorf("%s is already running", s.tool.name)
	default:
		if err = cmd.Start(); err == nil {
			s.tool = t
		}
	}
	s.mu.Unlock() // publish takes the mutex: it must not be held here
	if err != nil {
		return err
	}
	s.event(name, label)
	s.logf("$ %s", strings.Join(args, " "))

	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			onLine(append([]byte(nil), sc.Bytes()...))
		}
	}()
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			s.logf("%s", sc.Text())
		}
	}()
	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		if s.tool == t {
			s.tool = nil
		}
		s.mu.Unlock()
		status := "finished"
		if err != nil {
			status = "stopped (" + err.Error() + ")"
		}
		s.event(name, name+" "+status)
		s.publish(map[string]any{"type": "tool", "name": name, "running": false})
		close(t.done)
	}()
	s.publish(map[string]any{"type": "tool", "name": name, "running": true})
	return nil
}

// handleStop interrupts the running tool. The process lives inside the
// container, so it receives SIGINT there; killing the docker client would
// leave it running.
func (s *server) handleStop(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	t := s.tool
	s.mu.Unlock()
	if t == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	exec.Command("docker", "exec", "gerador", "pkill", "-INT", t.name).Run()
	select {
	case <-t.done:
	case <-time.After(5 * time.Second):
		exec.Command("docker", "exec", "gerador", "pkill", "-KILL", t.name).Run()
	}
	w.WriteHeader(http.StatusNoContent)
}

type faultParams struct {
	Kind  string `json:"kind"`  // latency, nolatency, partition, heal, kill, start
	Delay string `json:"delay"` // latency: e.g. "50ms"
	Node  string `json:"node"`  // kill, start: n1, n2 or n3
}

var validDelay = regexp.MustCompile(`^[0-9]{1,4}ms$`)

func (s *server) handleFault(w http.ResponseWriter, r *http.Request) {
	var p faultParams
	if err := decode(r, &p); err != nil {
		fail(w, err)
		return
	}
	s.mu.Lock()
	up := s.system != ""
	s.mu.Unlock()
	if !up {
		fail(w, errors.New("no cluster is up"))
		return
	}
	var err error
	switch p.Kind {
	case "latency":
		if !validDelay.MatchString(p.Delay) {
			fail(w, errors.New("delay must be like 50ms"))
			return
		}
		s.event("fault", "delay "+p.Delay+" per direction between nodes")
		err = s.runScript("latencia_on.sh", append([]string{p.Delay}, nodeNames...)...)
	case "nolatency":
		s.event("fault", "delay removed")
		err = s.runScript("latencia_off.sh", nodeNames...)
	case "partition":
		s.event("fault", "n1 partitioned from n2 and n3")
		err = s.runScript("particao.sh")
	case "heal":
		s.event("fault", "partition healed")
		err = s.runScript("cura.sh")
	case "kill", "start":
		if !validNode(p.Node) {
			fail(w, errors.New("node must be n1, n2 or n3"))
			return
		}
		s.event("fault", p.Kind+" "+p.Node)
		out, e := exec.Command("docker", p.Kind, p.Node).CombinedOutput()
		if e != nil {
			err = fmt.Errorf("%v: %s", e, out)
		}
	default:
		fail(w, errors.New("unknown fault"))
		return
	}
	if err != nil {
		s.logf("%s: %v", p.Kind, err)
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validNode(n string) bool {
	for _, name := range nodeNames {
		if n == name {
			return true
		}
	}
	return false
}

// nodeState is what the page shows about one node.
type nodeState struct {
	Name      string `json:"name"`
	Running   bool   `json:"running"`
	Connected bool   `json:"connected"` // on the cluster network
	Role      string `json:"role"`      // leader, follower, primary, replica, down, unknown
	Detail    string `json:"detail"`
}

// pollNodes publishes the state of the nodes once per second while a
// cluster is up.
func (s *server) pollNodes() {
	for range time.Tick(time.Second) {
		s.mu.Lock()
		system := s.system
		s.mu.Unlock()
		if system == "" {
			continue
		}
		states := make([]nodeState, len(nodeNames))
		var wg sync.WaitGroup
		for i, name := range nodeNames {
			wg.Add(1)
			go func() {
				defer wg.Done()
				states[i] = s.inspect(system, name)
			}()
		}
		wg.Wait()
		s.publish(map[string]any{"type": "nodes", "nodes": states})
	}
}

func (s *server) inspect(system, name string) nodeState {
	st := nodeState{Name: name, Role: "down"}
	out, err := exec.Command("docker", "inspect", "-f",
		`{{.State.Running}} {{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}`, name).Output()
	if err != nil {
		st.Detail = "no container"
		return st
	}
	fields := strings.Fields(string(out))
	st.Running = len(fields) > 0 && fields[0] == "true"
	for _, f := range fields[1:] {
		if f == "lab_cluster" {
			st.Connected = true
		}
	}
	if !st.Running {
		return st
	}
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	switch system {
	case "etcd":
		s.inspectEtcd(ctx, &st)
	case "redis":
		s.inspectRedis(ctx, &st)
	}
	return st
}

func (s *server) inspectEtcd(ctx context.Context, st *nodeState) {
	ep := "http://" + s.clientIP(st.Name) + ":2379"
	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{ep}, DialTimeout: 500 * time.Millisecond, Logger: zap.NewNop(), MaxUnaryRetries: 1})
	if err != nil {
		st.Role, st.Detail = "unknown", err.Error()
		return
	}
	defer cli.Close()
	resp, err := cli.Status(ctx, ep)
	if err != nil {
		st.Role, st.Detail = "unknown", "no answer"
		return
	}
	if resp.Leader == resp.Header.MemberId {
		st.Role = "leader"
	} else if resp.Leader == 0 {
		st.Role = "no leader"
	} else {
		st.Role = "follower"
	}
	st.Detail = fmt.Sprintf("term %d, index %d", resp.RaftTerm, resp.RaftIndex)
}

func (s *server) inspectRedis(ctx context.Context, st *nodeState) {
	cli := redis.NewClient(&redis.Options{Addr: s.clientIP(st.Name) + ":6379", DialTimeout: 500 * time.Millisecond, ReadTimeout: 500 * time.Millisecond, MaxRetries: -1, DisableIdentity: true})
	defer cli.Close()
	info, err := cli.Info(ctx, "replication").Result()
	if err != nil {
		st.Role, st.Detail = "unknown", "no answer"
		return
	}
	get := func(key string) string {
		for _, line := range strings.Split(info, "\n") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(line), key+":"); ok {
				return v
			}
		}
		return ""
	}
	switch get("role") {
	case "master":
		st.Role = "primary"
		st.Detail = get("connected_slaves") + " replicas, offset " + get("master_repl_offset")
	case "slave":
		st.Role = "replica"
		st.Detail = "link " + get("master_link_status") + ", offset " + get("slave_repl_offset")
	default:
		st.Role = "unknown"
	}
}
