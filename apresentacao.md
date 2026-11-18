# Roteiro da apresentação

Antes da aula: `lab/scripts/down.sh`; `go run ./cmd/viz` aberto em http://localhost:8080; `relatorio.md` aberto com as figuras; notebook na tomada, nada mais pesado rodando. As imagens Docker já estão baixadas, então subir um cluster leva 15 s (etcd) ou 20 s (Redis).

Tempo total: 12 a 15 min, mais perguntas.

## 1. O que foi medido (1 min)

Dois armazenamentos chave-valor com 3 nós: etcd (Raft, escrita confirmada por quórum com `fsync`, leitura linearizável) e Redis com duas réplicas assíncronas (escrita confirmada pelo primário, leitura na réplica). Mesma carga nos dois: N clientes em laço fechado, 80% GET e 20% PUT em 10.000 chaves, valores de 256 B, timeout de 2 s, semente fixa. Seis experimentos, 3 repetições cada, numa máquina só, com cada nó preso a um núcleo. Mostrar a tabela da Seção 1 do relatório.

## 2. E2 ao vivo no etcd (3 min)

Na página: `Start clean cluster` com etcd. Enquanto sobe, dizer o que a página mostra: requisições por segundo por nó, erros, p50 e p99, papel de cada nó.

Carga: N = 16, 60 s, warm-up 5 s, `Start load` (o E2 do relatório tem 3 min e 30 s de aquecimento; aqui é só para ver os mesmos números). Apontar: vazão em torno de 8 mil req/s, p50 de 1,4 ms, p99 de 10 ms, três nós respondendo igual, líder marcado no painel de nós. Comparar com a Seção 4 do relatório: mesma ordem de grandeza.

Ainda com a carga rodando, `docker kill` no nó marcado como líder. Apontar o buraco de cerca de 1 s e o novo líder no painel. `docker start` no mesmo nó: ele volta como seguidor sem erro. Isso é o E6: failover de 1,1 s, fixado pelo timeout de eleição de 1 s do Raft.

## 3. E2 e stale reads ao vivo no Redis (3 min)

`Start clean cluster` com Redis. Carga: N = 8, 60 s. Apontar: cerca de 300 mil req/s, p99 abaixo de 0,1 ms, n1 só com escritas e n2/n3 só com leituras. Dizer que o platô do E1 (530 mil) é o limite do gerador, não do Redis (nós a meio núcleo).

Com a carga terminada: `Start stale reads` com 10.000 pares, PUT n1, GET n3. Resultado: perto de 0%. Então `Add delay between nodes` (50 ms) e repetir: 100% stale. Explicar: a escrita é confirmada antes de chegar à réplica; a janela de leitura obsoleta é a latência entre nós. No etcd a mesma coisa dá 0% nas duas condições (Seção 5).

## 4. Resultados do relatório (4 min)

Mostrar as figuras, uma frase cada:

- E1 (`e1-saturation.png`): dois eixos log; etcd de 3 mil a 44 mil req/s com p99 subindo de 1 a 64 ms, sem platô até N=512 (agrupamento de propostas por `fsync`); Redis em platô a partir de N=64. N de referência: 16 no etcd, 8 no Redis.
- E4 (`e2e4-percentiles.png`): com RTT de 100 ms entre nós, o etcd vira múltiplo do RTT: PUT 100 ms no líder e 200 nos seguidores, GET 100 e 300; vazão cai 127 vezes. Redis não muda, porque o cliente fala só com o primário.
- E5 (`e5-etcd.png`, `e5-redis.png`): n1 isolado para de responder no etcd (C), ninguém para no Redis (A, com leituras velhas nas réplicas). Depois da cura: 0 divergência, 0 escrita perdida nos dois.
- E6 (`e6-*.png`): etcd 1,1 s para o líder e 2 ms para um seguidor; Redis sem Sentinel fica sem escritas até o primário voltar.

## 5. Conclusão (1 min)

As diferenças de ordem de grandeza vêm do modelo: rodadas de mensagens ao quórum (E4), leituras obsoletas (E3), C contra A (E5). Os valores absolutos vêm da implementação: `fsync`, agrupamento, timeout de eleição, single-thread do Redis, e do ambiente de uma máquina só.

## Perguntas prováveis

- Por que N=16 e não o N=1 que a razão vazão/p99 indica? N=1 é o ponto sem contenção; entre N≥2 a razão é estável e máxima em 16; o E5 precisa de pelo menos um cliente por nó.
- Por que a leitura do etcd é tão cara quanto a escrita? ReadIndex: confirma a liderança com o quórum e espera o apply. A leitura `serializable` evita isso e, em teste, deu 19% de stale reads.
- Por que não Sentinel? O apêndice permite a topologia sem failover, desde que declarada; o relatório diz o que mudaria com Sentinel.
- Por que o Redis não satura? Gerador e pilha de rede do host a 75–93%; nós a meio núcleo; registrado conforme o enunciado pede.
- Como o `check` acha escritas perdidas? Compara o valor armazenado de cada chave com o último PUT confirmado que começou depois do fim do PUT que gravou esse valor.
