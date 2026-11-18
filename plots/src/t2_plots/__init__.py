"""Figures of the report, from the CSV files written by cmd/analyze.

    uv run plots                 # every figure that has data, into ../figures/
    uv run plots -r ../results   # another results directory

Figures: e1 (p99 latency against throughput, one series per system), e2e4
(latency percentiles of E2 and E4 side by side), e3 (stale read rate), e5 and
e6 (completions and errors per second around the fault, one panel per run).
"""

import argparse
import sys
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt  # noqa: E402
import pandas as pd  # noqa: E402

SYSTEM = {"etcd": "#2a78d6", "redis": "#eb6834"}
NODE = {"n1": "#2a78d6", "n2": "#1baf7a", "n3": "#6250d6"}
OP = {"GET": "#2a78d6", "PUT": "#eb6834"}
ERR = "#e34948"
INK, INK2, GRID = "#0b0b0b", "#52514e", "#e1e0d9"

plt.rcParams.update({
    "font.family": "sans-serif",
    "font.size": 9,
    "axes.edgecolor": GRID,
    "axes.labelcolor": INK2,
    "axes.titlecolor": INK,
    "axes.titlesize": 10,
    "axes.titleweight": "bold",
    "axes.grid": True,
    "grid.color": GRID,
    "grid.linewidth": 0.6,
    "axes.spines.top": False,
    "axes.spines.right": False,
    "xtick.color": INK2,
    "ytick.color": INK2,
    "legend.frameon": False,
    "figure.dpi": 150,
    "savefig.dpi": 200,
    "savefig.bbox": "tight",
})


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("-r", "--results", default=Path(__file__).resolve().parents[3] / "results", type=Path)
    ap.add_argument("-o", "--out", default=Path(__file__).resolve().parents[3] / "figures", type=Path)
    ap.add_argument("figures", nargs="*", help="subset of: e1 e2e4 e3 e5 e6 (default: all)")
    args = ap.parse_args()
    args.out.mkdir(parents=True, exist_ok=True)
    wanted = args.figures or ["e1", "e2e4", "e3", "e5", "e6"]
    makers = {"e1": e1, "e2e4": e2e4, "e3": e3, "e5": e5, "e6": e6}
    for name in wanted:
        try:
            paths = makers[name](args.results, args.out)
        except FileNotFoundError as exc:
            print(f"{name}: skipped, {exc.filename} not found (run cmd/analyze first)", file=sys.stderr)
            continue
        for p in paths:
            print(p)


def e1(results: Path, out: Path) -> list[Path]:
    df = pd.read_csv(results / "e1" / "e1.csv")
    fig, ax = plt.subplots(figsize=(7, 4.2))
    for system, g in df.groupby("system", sort=True):
        g = g.sort_values("threads")
        c = SYSTEM.get(system, INK)
        ax.plot(g.throughput_median, g.p99_ms_median, "-o", color=c, lw=1.5, ms=4, label=system)
        # Range over the repetitions, both axes.
        ax.errorbar(g.throughput_median, g.p99_ms_median,
                    xerr=[g.throughput_median - g.throughput_min, g.throughput_max - g.throughput_median],
                    yerr=[g.p99_ms_median - g.p99_ms_min, g.p99_ms_max - g.p99_ms_median],
                    fmt="none", ecolor=c, elinewidth=0.6, alpha=0.6, capsize=2)
        for _, r in g.iterrows():
            ax.annotate(f"N={r.threads:.0f}", (r.throughput_median, r.p99_ms_median),
                        textcoords="offset points", xytext=(5, -9 if system == "etcd" else 6),
                        fontsize=7, color=INK2)
        opt = g[g.optimal]
        if len(opt):
            ax.plot(opt.throughput_median, opt.p99_ms_median, "o", ms=11, mfc="none", mec=c, mew=1.5)
            ax.annotate("optimal", (opt.throughput_median.iloc[0], opt.p99_ms_median.iloc[0]),
                        textcoords="offset points", xytext=(8, 8), fontsize=7, color=c)
        plateau = g[g.plateau]
        if len(plateau) >= 2:
            ax.axvline(plateau.throughput_median.max(), color=c, lw=0.6, ls=":", alpha=0.7)
    ax.set_yscale("log")
    ax.set_xlabel("effective throughput (successful requests per second)")
    ax.set_ylabel("p99 latency (ms), log scale")
    ax.set_title("E1. Latency against throughput, one point per N; circle: optimal point, dotted line: plateau")
    ax.legend(loc="upper left")
    ax.grid(True, which="minor", axis="y", alpha=0.4)
    path = out / "e1-saturation.png"
    fig.savefig(path)
    plt.close(fig)
    return [path]


def e2e4(results: Path, out: Path) -> list[Path]:
    df = pd.read_csv(results / "e2" / "e2e4.csv")
    pct = ["p50 ms", "p95 ms", "p99 ms"]
    systems = sorted(df.system.unique())
    fig, axes = plt.subplots(1, len(systems), figsize=(4.2 * len(systems), 4), sharey=False, squeeze=False)
    for ax, system in zip(axes[0], systems):
        g = df[df.system == system]
        labels, width, x = [], 0.2, 0
        for op in ["GET", "PUT"]:
            for exp, hatch in [("e2", ""), ("e4", "////")]:
                row = g[(g.op == op) & (g.experiment == exp)]
                if row.empty:
                    continue
                vals = [median_of(row.iloc[0][p]) for p in pct]
                xs = [x + i * width for i in range(3)]
                ax.bar(xs, vals, width=width * 0.9, color=OP[op], hatch=hatch, edgecolor="white", lw=0.5,
                       label=f"{op} {'E2 (normal)' if exp == 'e2' else 'E4 (RTT 100 ms)'}")
                for xi, v in zip(xs, vals):
                    ax.annotate(f"{v:.2g}", (xi, v), ha="center", va="bottom", fontsize=6.5, color=INK2,
                                textcoords="offset points", xytext=(0, 1))
                labels.append((x + width, f"{op} {exp.upper()}"))
                x += 0.8
        ax.set_yscale("log")
        ax.set_xticks([p for p, _ in labels])
        ax.set_xticklabels([l for _, l in labels])
        ax.set_title(f"{system}: N = {int(g.N.iloc[0])}")
        ax.set_ylabel("latency (ms), log scale; bars: p50, p95, p99")
        ax.grid(axis="x", visible=False)
    fig.suptitle("E2 and E4. Latency per percentile with the same N, normal network and RTT of 100 ms between nodes", fontsize=10)
    path = out / "e2e4-percentiles.png"
    fig.savefig(path)
    plt.close(fig)
    return [path]


def median_of(cell) -> float:
    """The median from a 'median (min to max)' cell, or the number itself."""
    return float(str(cell).split(" ")[0])


def e3(results: Path, out: Path) -> list[Path]:
    df = pd.read_csv(results / "e3" / "e3.csv")
    fig, ax = plt.subplots(figsize=(5.5, 3.6))
    conds = ["normal", "delay"]
    width = 0.35
    for i, (system, g) in enumerate(sorted(df.groupby("system"))):
        vals = []
        for cond in conds:
            row = g[g.condition == cond]
            vals.append(float(str(row.iloc[0]["stale rate"]).split("%")[0]) if len(row) else 0.0)
        xs = [j + (i - 0.5) * width for j in range(len(conds))]
        ax.bar(xs, vals, width=width * 0.9, color=SYSTEM.get(system, INK), label=system)
        for xi, v in zip(xs, vals):
            ax.annotate(f"{v:.2f}%", (xi, v), ha="center", va="bottom", fontsize=7, color=INK2,
                        textcoords="offset points", xytext=(0, 1))
    ax.set_xticks(range(len(conds)))
    ax.set_xticklabels(["original network", "RTT 100 ms between nodes"])
    ax.set_ylabel("stale reads (% of successful GETs)")
    ax.set_ylim(0, 105)
    ax.set_title("E3. Stale reads: PUT on n1, then GET on n3")
    ax.legend()
    ax.grid(axis="x", visible=False)
    path = out / "e3-stale-reads.png"
    fig.savefig(path)
    plt.close(fig)
    return [path]


def e5(results: Path, out: Path) -> list[Path]:
    return fault_timeline(results / "e5" / "e5-timeline.csv", out, "e5", "partition of n1", "heal")


def e6(results: Path, out: Path) -> list[Path]:
    return fault_timeline(results / "e6" / "e6-timeline.csv", out, "e6", "kill", "restart")


def fault_timeline(csv: Path, out: Path, exp: str, fault: str, recovery: str) -> list[Path]:
    """One figure per system (and variant), with two panels per repetition:
    successful requests per second by node, and errors per second."""
    df = pd.read_csv(csv)
    df["variant"] = df["variant"].fillna("")
    paths = []
    for (system, variant), g in df.groupby(["system", "variant"], sort=True):
        reps = sorted(g.rep.unique())
        fig, axes = plt.subplots(2 * len(reps), 1, figsize=(8, 3.4 * len(reps)), sharex=True, squeeze=False,
                                 gridspec_kw={"height_ratios": [3, 1] * len(reps), "hspace": 0.08})
        for i, rep in enumerate(reps):
            ax, axe = axes[2 * i, 0], axes[2 * i + 1, 0]
            r = g[g.rep == rep]
            nodes = r[r.node != "all"].groupby(["t_rel_s", "node"]).ok.sum().unstack(fill_value=0)
            bottom = None
            for node in ["n1", "n2", "n3"]:
                if node not in nodes:
                    continue
                ax.bar(nodes.index, nodes[node], bottom=bottom, width=1, color=NODE[node], label=node)
                bottom = nodes[node] if bottom is None else bottom + nodes[node]
            errs = r[r.node == "all"].groupby("t_rel_s")[["timeouts", "failed"]].sum().sum(axis=1)
            axe.bar(errs.index, errs.values, width=1, color=ERR)
            axe.set_ylim(0, max(errs.max() * 1.15, 1))
            rec = r[r.phase == "after"].t_rel_s.min()
            for a in (ax, axe):
                a.axvline(0, color=INK, lw=0.8, ls="--")
                if pd.notna(rec):
                    a.axvline(rec, color=INK, lw=0.8, ls=":")
                a.grid(axis="x", visible=False)
            ax.annotate(fault, (0, ax.get_ylim()[1]), fontsize=7, color=INK2, va="top",
                        textcoords="offset points", xytext=(3, -2))
            if pd.notna(rec):
                ax.annotate(recovery, (rec, ax.get_ylim()[1]), fontsize=7, color=INK2, va="top",
                            textcoords="offset points", xytext=(3, -2))
            ax.set_ylabel("successful req/s")
            axe.set_ylabel("errors/s", color=INK2)
            ax.set_title(f"{system}{' ' + variant if variant else ''} {rep}", loc="left")
            warm = r[r.phase == "before"].t_rel_s.min()
            ax.set_xlim(warm, r.t_rel_s.max() + 1)
            ax.tick_params(axis="x", length=0)
        axes[-1, 0].set_xlabel("seconds from the fault")
        h1, l1 = axes[0, 0].get_legend_handles_labels()
        fig.legend(h1 + [plt.Rectangle((0, 0), 1, 1, color=ERR)], l1 + ["errors (timeouts and failures)"],
                   loc="upper right", ncol=4, fontsize=8)
        title = {"e5": "E5. Partition of n1 (clients pinned to one node)", "e6": "E6. Node failure"}[exp]
        fig.suptitle(f"{title}: {system}{' ' + variant if variant else ''}", fontsize=10, x=0.01, ha="left")
        fig.tight_layout(rect=(0, 0, 1, 0.93))
        path = out / f"{exp}-{system}{'-' + variant if variant else ''}.png"
        fig.savefig(path)
        plt.close(fig)
        paths.append(path)
    return paths
