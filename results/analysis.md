## E1: etcd

| N | reps | req/s | range | p99 ms | range | req/s per ms | GET p99 | PUT p99 | errors | CPU n1 | CPU n2 | CPU n3 | generator |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 * | 3 | 3277 | 3257 to 3310 | 1.25 | 1.25 to 1.26 | 2623 | 0.60 | 6.21 | 0 | 0.22 | 0.22 | 0.22 | 3% |
| 2 | 3 | 4242 | 4222 to 4256 | 5.18 | 4.97 to 5.42 | 819 | 1.50 | 6.91 | 0 | 0.34 | 0.27 | 0.26 | 3% |
| 4 | 3 | 5370 | 5352 to 5379 | 6.63 | 6.57 to 6.63 | 810 | 6.25 | 7.46 | 0 | 0.30 | 0.38 | 0.30 | 4% |
| 8 | 3 | 6556 | 6505 to 6582 | 7.89 | 7.84 to 7.94 | 831 | 7.81 | 8.16 | 0 | 0.30 | 0.30 | 0.30 | 4% |
| 16 | 3 | 8316 | 8300 to 8426 | 9.76 | 9.41 to 9.86 | 852 | 9.83 | 9.22 | 0 | 0.29 | 0.30 | 0.30 | 4% |
| 32 | 3 | 10885 | 10671 to 11022 | 15.60 | 15.14 to 16.63 | 698 | 15.88 | 14.37 | 0 | 0.33 | 0.32 | 0.33 | 6% |
| 64 | 3 | 15494 | 15383 to 16121 | 20.77 | 18.32 to 21.01 | 746 | 21.38 | 16.88 | 0 | 0.42 | 0.37 | 0.36 | 8% |
| 128 | 3 | 21281 | 21109 to 21543 | 31.29 | 30.10 to 34.71 | 680 | 33.01 | 24.57 | 0 | 0.49 | 0.45 | 0.45 | 10% |
| 256 | 3 | 30181 | 29140 to 31100 | 51.15 | 43.27 to 54.65 | 590 | 54.47 | 37.65 | 0 | 0.63 | 0.63 | 0.67 | 14% |
| 512 | 3 | 44125 | 42686 to 45588 | 64.35 | 59.24 to 80.00 | 686 | 70.55 | 44.50 | 0 | 0.85 | 0.86 | 0.92 | 19% |
| 1024 | 3 | 56598 | 56062 to 57611 | 98.81 | 98.62 to 102.34 | 573 | 108.87 | 71.26 | 0 | 1.01 | 1.02 | 1.01 | 21% |
| 2048 | 3 | 64604 | 63810 to 65508 | 145.82 | 145.55 to 146.07 | 443 | 155.31 | 103.26 | 0 | 1.09 | 1.09 | 1.10 | 22% |

Optimal point (*): N = 1, 3277 req/s at p99 1.25 ms (2623 req/s per ms).
Maximum sustained throughput: 64604 req/s. Plateau (within 5% of it) from N = 2048, 1 point(s).
WARNING: fewer than two points on the plateau: extend the curve with larger N

## E1: redis

| N | reps | req/s | range | p99 ms | range | req/s per ms | GET p99 | PUT p99 | errors | CPU n1 | CPU n2 | CPU n3 | generator |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 3 | 87649 | 85870 to 89790 | 0.02 | 0.02 to 0.02 | 4349633 | 0.02 | 0.02 | 0 | 0.11 | 0.16 | 0.16 | 5% |
| 2 | 3 | 129825 | 124297 to 130876 | 0.03 | 0.03 to 0.03 | 4111234 | 0.03 | 0.04 | 0 | 0.16 | 0.24 | 0.24 | 10% |
| 4 | 3 | 195043 | 194759 to 198947 | 0.04 | 0.04 to 0.04 | 4882289 | 0.04 | 0.05 | 0 | 0.26 | 0.38 | 0.39 | 25% |
| 8 * | 3 | 304618 | 299517 to 311919 | 0.05 | 0.05 to 0.05 | 6460074 | 0.04 | 0.05 | 0 | 0.41 | 0.57 | 0.56 | 53% |
| 16 | 3 | 426206 | 403147 to 428014 | 0.07 | 0.07 to 0.08 | 5736366 | 0.07 | 0.07 | 0 | 0.52 | 0.66 | 0.66 | 70% |
| 32 | 3 | 488276 | 470527 to 503698 | 0.17 | 0.15 to 0.17 | 2941332 | 0.17 | 0.12 | 0 | 0.57 | 0.65 | 0.65 | 70% |
| 64 | 3 | 515519 | 500668 to 528178 | 0.32 | 0.30 to 0.32 | 1633177 | 0.33 | 0.23 | 0 | 0.58 | 0.65 | 0.65 | 70% |
| 128 | 3 | 521994 | 512369 to 536778 | 0.65 | 0.64 to 0.70 | 799626 | 0.68 | 0.38 | 0 | 0.59 | 0.65 | 0.65 | 71% |
| 256 | 3 | 531359 | 518142 to 540163 | 1.33 | 1.27 to 1.35 | 399962 | 1.36 | 0.58 | 0 | 0.59 | 0.65 | 0.65 | 72% |
| 512 | 3 | 509345 | 495734 to 510357 | 3.13 | 3.13 to 3.22 | 162541 | 3.21 | 0.46 | 0 | 0.55 | 0.66 | 0.65 | 69% |

Optimal point (*): N = 8, 304618 req/s at p99 0.05 ms (6460074 req/s per ms).
Maximum sustained throughput: 531359 req/s. Plateau (within 5% of it) from N = 64, 4 point(s).

written: results/e1/e1.csv
## E2 and E4: latency per percentile

| system | experiment | N | reps | op | req/s | p50 ms | p95 ms | p99 ms | errors | generator |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| etcd | e2 | 16 | 3 | GET | 6601 (6556 to 6776) | 1.45 (1.45 to 1.46) | 5.53 (4.70 to 5.89) | 10.47 (9.43 to 10.62) | 0 | 5% |
| etcd | e2 | 16 | 3 | PUT | 1655 (1644 to 1697) | 1.44 (1.43 to 1.45) | 5.86 (4.51 to 6.18) | 9.56 (8.91 to 9.65) | 0 | 5% |
| redis | e2 | 8 | 3 | GET | 248005 (232129 to 249504) | 0.02 (0.02 to 0.02) | 0.03 (0.03 to 0.04) | 0.04 (0.04 to 0.05) | 0 | 53% |
| redis | e2 | 8 | 3 | PUT | 61990 (58019 to 62363) | 0.03 (0.03 to 0.03) | 0.04 (0.04 to 0.05) | 0.05 (0.05 to 0.06) | 0 | 53% |
| etcd | e4 | 16 | 3 | GET | 52 (51 to 52) | 298.82 (298.79 to 299.91) | 400.73 (400.70 to 400.74) | 401.14 (400.98 to 401.18) | 0 | 0% |
| etcd | e4 | 16 | 3 | PUT | 13 (13 to 13) | 200.68 (200.68 to 200.69) | 201.53 (201.37 to 201.55) | 202.18 (202.10 to 202.82) | 0 | 0% |
| redis | e4 | 8 | 3 | GET | 254737 (253928 to 255872) | 0.02 (0.02 to 0.02) | 0.04 (0.04 to 0.04) | 0.08 (0.07 to 0.08) | 0 | 54% |
| redis | e4 | 8 | 3 | PUT | 63677 (63473 to 63961) | 0.02 (0.02 to 0.02) | 0.03 (0.03 to 0.03) | 0.04 (0.04 to 0.04) | 0 | 54% |

E4 etcd, RTT between nodes: rtt min/avg/max/mdev = 100.015/100.160/100.286/0.103 ms

E4 redis, RTT between nodes: rtt min/avg/max/mdev = 100.074/100.288/100.421/0.126 ms

written: results/e2/e2e4.csv

## E3: stale reads

| system | condition | read | nodes | reps | pairs | stale rate | stale / GETs | max lag | PUT errors | GET errors | PUT p50 ms | PUT p99 ms | GET p50 ms | GET p99 ms |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| etcd | normal | linearizable | PUT n1, GET n3 | 3 | 10000 | 0.00% | 0 / 30000 | 0 | 0 | 0 | 0.708 (0.701 to 0.718) | 1.995 (1.540 to 7.107) | 0.157 (0.128 to 0.159) | 0.833 (0.582 to 0.985) |
| etcd | delay | linearizable | PUT n1, GET n3 | 3 | 10000 | 0.00% | 0 / 30000 | 0 | 0 | 0 | 101.987 (100.875 to 202.041) | 115.805 (101.954 to 217.907) | 298.981 (198.821 to 300.121) | 304.595 (200.246 to 308.512) |
| redis | normal | any node | PUT n1, GET n3 | 3 | 10000 | 2.60% (1.72% to 10.63%) | 1495 / 30000 | 1 | 0 | 0 | 0.039 (0.022 to 0.047) | 0.175 (0.064 to 1.722) | 0.027 (0.015 to 0.038) | 0.154 (0.052 to 0.641) |
| redis | delay | any node | PUT n1, GET n3 | 3 | 10000 | 100.00% | 30000 / 30000 | 500 | 0 | 0 | 0.031 (0.021 to 0.043) | 0.225 (0.105 to 2.545) | 0.030 (0.022 to 0.045) | 0.206 (0.110 to 2.878) |

etcd with delay, RTT between nodes: rtt min/avg/max/mdev = 100.022/100.132/100.256/0.093 ms

redis with delay, RTT between nodes: rtt min/avg/max/mdev = 100.071/100.216/100.339/0.100 ms

written: results/e3/e3.csv

## E5: partition of n1, requests per node and phase

| system | variant | rep | coordinator | fault | phase | s | node | GET ok | GET err | PUT ok | PUT err | silent s |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| etcd |  | rep1 | n3 | partition of n1 | before | 29 | n1 | 55849 | 0 | 14132 | 0 | 1 |
| etcd |  | rep1 | n3 | partition of n1 | before | 29 | n2 | 50887 | 0 | 13043 | 0 | 0 |
| etcd |  | rep1 | n3 | partition of n1 | before | 29 | n3 | 86352 | 0 | 21678 | 0 | 0 |
| etcd |  | rep1 | n3 | partition of n1 | during | 60 | n1 | 2589 | 134 | 675 | 40 | 29 |
| etcd |  | rep1 | n3 | partition of n1 | during | 60 | n2 | 146713 | 0 | 36799 | 0 | 0 |
| etcd |  | rep1 | n3 | partition of n1 | during | 60 | n3 | 216329 | 0 | 54118 | 0 | 0 |
| etcd |  | rep1 | n3 | partition of n1 | after | 60 | n1 | 119971 | 0 | 29779 | 0 | 0 |
| etcd |  | rep1 | n3 | partition of n1 | after | 60 | n2 | 105037 | 0 | 26418 | 0 | 0 |
| etcd |  | rep1 | n3 | partition of n1 | after | 60 | n3 | 180347 | 0 | 44952 | 0 | 0 |
| etcd |  | rep2 | n3 | partition of n1 | before | 29 | n1 | 53534 | 0 | 13524 | 0 | 1 |
| etcd |  | rep2 | n3 | partition of n1 | before | 29 | n2 | 48560 | 0 | 12437 | 0 | 0 |
| etcd |  | rep2 | n3 | partition of n1 | before | 29 | n3 | 83166 | 0 | 20949 | 0 | 0 |
| etcd |  | rep2 | n3 | partition of n1 | during | 60 | n1 | 0 | 152 | 0 | 28 | 30 |
| etcd |  | rep2 | n3 | partition of n1 | during | 60 | n2 | 149857 | 0 | 37639 | 0 | 0 |
| etcd |  | rep2 | n3 | partition of n1 | during | 60 | n3 | 220483 | 0 | 55141 | 0 | 0 |
| etcd |  | rep2 | n3 | partition of n1 | after | 60 | n1 | 121232 | 0 | 30182 | 4 | 0 |
| etcd |  | rep2 | n3 | partition of n1 | after | 60 | n2 | 106817 | 0 | 26862 | 0 | 0 |
| etcd |  | rep2 | n3 | partition of n1 | after | 60 | n3 | 185198 | 0 | 46113 | 0 | 0 |
| etcd |  | rep3 | n3 | partition of n1 | before | 29 | n1 | 55881 | 0 | 14123 | 0 | 1 |
| etcd |  | rep3 | n3 | partition of n1 | before | 29 | n2 | 49976 | 0 | 12821 | 0 | 0 |
| etcd |  | rep3 | n3 | partition of n1 | before | 29 | n3 | 86302 | 0 | 21678 | 0 | 0 |
| etcd |  | rep3 | n3 | partition of n1 | during | 60 | n1 | 2461 | 137 | 646 | 37 | 29 |
| etcd |  | rep3 | n3 | partition of n1 | during | 60 | n2 | 150249 | 0 | 37706 | 0 | 0 |
| etcd |  | rep3 | n3 | partition of n1 | during | 60 | n3 | 221435 | 0 | 55387 | 0 | 0 |
| etcd |  | rep3 | n3 | partition of n1 | after | 60 | n1 | 126556 | 0 | 31458 | 0 | 0 |
| etcd |  | rep3 | n3 | partition of n1 | after | 60 | n2 | 110017 | 0 | 27604 | 0 | 0 |
| etcd |  | rep3 | n3 | partition of n1 | after | 60 | n3 | 190316 | 0 | 47435 | 0 | 0 |
| redis |  | rep1 | n1 | partition of n1 | before | 30 | n1 | 2326776 | 0 | 1791975 | 0 | 0 |
| redis |  | rep1 | n1 | partition of n1 | before | 30 | n2 | 2886491 | 0 | 0 | 0 | 0 |
| redis |  | rep1 | n1 | partition of n1 | before | 30 | n3 | 1962165 | 0 | 0 | 0 | 0 |
| redis |  | rep1 | n1 | partition of n1 | during | 60 | n1 | 5835357 | 0 | 4037875 | 0 | 0 |
| redis |  | rep1 | n1 | partition of n1 | during | 60 | n2 | 6144508 | 0 | 0 | 0 | 0 |
| redis |  | rep1 | n1 | partition of n1 | during | 60 | n3 | 4170541 | 0 | 0 | 0 | 0 |
| redis |  | rep1 | n1 | partition of n1 | after | 59 | n1 | 4602308 | 0 | 3530351 | 0 | 0 |
| redis |  | rep1 | n1 | partition of n1 | after | 59 | n2 | 5663290 | 3 | 0 | 0 | 0 |
| redis |  | rep1 | n1 | partition of n1 | after | 59 | n3 | 3856571 | 2 | 0 | 0 | 0 |
| redis |  | rep2 | n1 | partition of n1 | before | 30 | n1 | 2332847 | 0 | 1793428 | 0 | 0 |
| redis |  | rep2 | n1 | partition of n1 | before | 30 | n2 | 2884117 | 0 | 0 | 0 | 0 |
| redis |  | rep2 | n1 | partition of n1 | before | 30 | n3 | 1963409 | 0 | 0 | 0 | 0 |
| redis |  | rep2 | n1 | partition of n1 | during | 60 | n1 | 5754625 | 0 | 3984824 | 0 | 0 |
| redis |  | rep2 | n1 | partition of n1 | during | 60 | n2 | 6066364 | 0 | 0 | 0 | 0 |
| redis |  | rep2 | n1 | partition of n1 | during | 60 | n3 | 4117317 | 0 | 0 | 0 | 0 |
| redis |  | rep2 | n1 | partition of n1 | after | 59 | n1 | 4540961 | 0 | 3450769 | 0 | 0 |
| redis |  | rep2 | n1 | partition of n1 | after | 59 | n2 | 5500763 | 2 | 0 | 0 | 0 |
| redis |  | rep2 | n1 | partition of n1 | after | 59 | n3 | 3761249 | 2 | 0 | 0 | 0 |
| redis |  | rep3 | n1 | partition of n1 | before | 30 | n1 | 2317472 | 0 | 1781414 | 0 | 0 |
| redis |  | rep3 | n1 | partition of n1 | before | 30 | n2 | 2861716 | 0 | 0 | 0 | 0 |
| redis |  | rep3 | n1 | partition of n1 | before | 30 | n3 | 1954171 | 0 | 0 | 0 | 0 |
| redis |  | rep3 | n1 | partition of n1 | during | 60 | n1 | 5818667 | 0 | 4000556 | 0 | 0 |
| redis |  | rep3 | n1 | partition of n1 | during | 60 | n2 | 6024790 | 0 | 0 | 0 | 0 |
| redis |  | rep3 | n1 | partition of n1 | during | 60 | n3 | 4156853 | 0 | 0 | 0 | 0 |
| redis |  | rep3 | n1 | partition of n1 | after | 59 | n1 | 4634953 | 0 | 3535783 | 0 | 0 |
| redis |  | rep3 | n1 | partition of n1 | after | 59 | n2 | 5643458 | 3 | 0 | 0 | 0 |
| redis |  | rep3 | n1 | partition of n1 | after | 59 | n3 | 3864987 | 2 | 0 | 0 | 0 |

Convergence after the heal:

| system | rep | coordinator | divergent keys | lost writes | unknown values | in flight at the end | missing keys |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| etcd | rep1 | n3 | 0 | n1 0, n2 0, n3 0 | n1 0, n2 0, n3 0 | n1 4, n2 4, n3 4 | n1 0, n2 0, n3 0 |
| etcd | rep2 | n3 | 0 | n1 0, n2 0, n3 0 | n1 0, n2 0, n3 0 | n1 6, n2 6, n3 6 | n1 0, n2 0, n3 0 |
| etcd | rep3 | n3 | 0 | n1 0, n2 0, n3 0 | n1 0, n2 0, n3 0 | n1 1, n2 1, n3 1 | n1 0, n2 0, n3 0 |
| redis | rep1 | n1 | 0 | n1 0, n2 0, n3 0 | n1 0, n2 0, n3 0 | n1 2, n2 2, n3 2 | n1 0, n2 0, n3 0 |
| redis | rep2 | n1 | 0 | n1 0, n2 0, n3 0 | n1 0, n2 0, n3 0 | n1 0, n2 0, n3 0 | n1 0, n2 0, n3 0 |
| redis | rep3 | n1 | 0 | n1 0, n2 0, n3 0 | n1 0, n2 0, n3 0 | n1 1, n2 1, n3 1 | n1 0, n2 0, n3 0 |

written: results/e5/e5.csv

written: results/e5/e5-timeline.csv

## E6: node failure

| system | variant | reps | victims | write failover s | first writer after | errors during the failure | req/s during | req/s before | errors after restart | req/s after |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| etcd | leader | 3 | n3 n2 | 1.110 (0.872 to 1.262) | n1 n2 | 24 (21 to 32) | 8804 (8648 to 8834) | 8076 (8049 to 8223) | 0 | 8527 (8376 to 8584) |
| etcd | follower | 3 | n1 | 0.002 (0.001 to 0.004) | n3 | 20 | 9056 (8731 to 9454) | 8526 (8362 to 8571) | 0 | 8143 (8073 to 8436) |
| redis | leader | 3 | n1 | 60.666 (60.632 to 60.878) | n1 | 38097 (38045 to 38198) | 3483 (2563 to 10620) | 298950 (284013 to 300615) | 12 (11 to 68) | 321846 (320472 to 337562) |
| redis | follower | 3 | n2 | 0.000 (0.000 to 0.000) | n1 | 38 | 285170 (279701 to 288340) | 311311 (309187 to 312568) | 8 (8 to 9) | 311285 (310377 to 312912) |

Requests per node and phase:

| system | variant | rep | coordinator | fault | phase | s | node | GET ok | GET err | PUT ok | PUT err | silent s |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| etcd | leader | rep1 | n2 | kill n2 | before | 29 | n1 | 62338 | 0 | 15731 | 0 | 0 |
| etcd | leader | rep1 | n2 | kill n2 | before | 29 | n2 | 62337 | 2 | 15727 | 1 | 0 |
| etcd | leader | rep1 | n2 | kill n2 | before | 29 | n3 | 62341 | 0 | 15731 | 0 | 0 |
| etcd | leader | rep1 | n2 | kill n2 | during | 60 | n1 | 212134 | 1 | 52877 | 1 | 0 |
| etcd | leader | rep1 | n2 | kill n2 | during | 60 | n2 | 0 | 19 | 0 | 1 | 40 |
| etcd | leader | rep1 | n2 | kill n2 | during | 60 | n3 | 212141 | 1 | 52877 | 1 | 0 |
| etcd | leader | rep1 | n2 | kill n2 | after | 60 | n1 | 134484 | 0 | 33735 | 0 | 0 |
| etcd | leader | rep1 | n2 | kill n2 | after | 60 | n2 | 132812 | 0 | 33333 | 0 | 0 |
| etcd | leader | rep1 | n2 | kill n2 | after | 60 | n3 | 134474 | 0 | 33734 | 0 | 0 |
| etcd | leader | rep2 | n3 | kill n3 | before | 30 | n1 | 64518 | 7 | 16283 | 0 | 0 |
| etcd | leader | rep2 | n3 | kill n3 | before | 30 | n2 | 64515 | 1 | 16279 | 0 | 0 |
| etcd | leader | rep2 | n3 | kill n3 | before | 30 | n3 | 63787 | 2 | 16098 | 1 | 1 |
| etcd | leader | rep2 | n3 | kill n3 | during | 60 | n1 | 206898 | 0 | 51655 | 1 | 0 |
| etcd | leader | rep2 | n3 | kill n3 | during | 60 | n2 | 206896 | 0 | 51659 | 0 | 0 |
| etcd | leader | rep2 | n3 | kill n3 | during | 60 | n3 | 1404 | 13 | 358 | 7 | 39 |
| etcd | leader | rep2 | n3 | kill n3 | after | 59 | n1 | 134106 | 0 | 33584 | 0 | 0 |
| etcd | leader | rep2 | n3 | kill n3 | after | 59 | n2 | 134121 | 0 | 33588 | 0 | 0 |
| etcd | leader | rep2 | n3 | kill n3 | after | 59 | n3 | 134115 | 0 | 33584 | 0 | 0 |
| etcd | leader | rep3 | n3 | kill n3 | before | 29 | n1 | 63477 | 0 | 16014 | 0 | 0 |
| etcd | leader | rep3 | n3 | kill n3 | before | 29 | n2 | 63474 | 0 | 16012 | 0 | 0 |
| etcd | leader | rep3 | n3 | kill n3 | before | 29 | n3 | 63479 | 1 | 16015 | 1 | 0 |
| etcd | leader | rep3 | n3 | kill n3 | during | 60 | n1 | 211404 | 2 | 52734 | 1 | 0 |
| etcd | leader | rep3 | n3 | kill n3 | during | 60 | n2 | 211398 | 5 | 52732 | 1 | 0 |
| etcd | leader | rep3 | n3 | kill n3 | during | 60 | n3 | 0 | 17 | 0 | 6 | 40 |
| etcd | leader | rep3 | n3 | kill n3 | after | 60 | n1 | 138147 | 0 | 34652 | 0 | 0 |
| etcd | leader | rep3 | n3 | kill n3 | after | 60 | n2 | 138152 | 0 | 34652 | 0 | 0 |
| etcd | leader | rep3 | n3 | kill n3 | after | 60 | n3 | 135452 | 0 | 33989 | 0 | 0 |
| etcd | follower | rep1 | n2 | kill n1 | before | 29 | n1 | 64217 | 13 | 16195 | 3 | 0 |
| etcd | follower | rep1 | n2 | kill n1 | before | 29 | n2 | 66606 | 0 | 16820 | 0 | 0 |
| etcd | follower | rep1 | n2 | kill n1 | before | 29 | n3 | 66605 | 0 | 16819 | 0 | 0 |
| etcd | follower | rep1 | n2 | kill n1 | during | 60 | n1 | 0 | 17 | 0 | 3 | 40 |
| etcd | follower | rep1 | n2 | kill n1 | during | 60 | n2 | 217500 | 0 | 54170 | 0 | 0 |
| etcd | follower | rep1 | n2 | kill n1 | during | 60 | n3 | 217505 | 0 | 54171 | 0 | 0 |
| etcd | follower | rep1 | n2 | kill n1 | after | 60 | n1 | 134334 | 0 | 33716 | 0 | 0 |
| etcd | follower | rep1 | n2 | kill n1 | after | 60 | n2 | 135160 | 0 | 33907 | 0 | 0 |
| etcd | follower | rep1 | n2 | kill n1 | after | 60 | n3 | 135154 | 0 | 33908 | 0 | 0 |
| etcd | follower | rep2 | n3 | kill n1 | before | 29 | n1 | 64039 | 11 | 16158 | 3 | 0 |
| etcd | follower | rep2 | n3 | kill n1 | before | 29 | n2 | 67236 | 0 | 16954 | 0 | 0 |
| etcd | follower | rep2 | n3 | kill n1 | before | 29 | n3 | 67232 | 0 | 16952 | 0 | 0 |
| etcd | follower | rep2 | n3 | kill n1 | during | 60 | n1 | 0 | 14 | 0 | 6 | 40 |
| etcd | follower | rep2 | n3 | kill n1 | during | 60 | n2 | 209624 | 0 | 52311 | 0 | 0 |
| etcd | follower | rep2 | n3 | kill n1 | during | 60 | n3 | 209621 | 0 | 52309 | 0 | 0 |
| etcd | follower | rep2 | n3 | kill n1 | after | 60 | n1 | 128976 | 0 | 32358 | 0 | 0 |
| etcd | follower | rep2 | n3 | kill n1 | after | 60 | n2 | 130829 | 0 | 32776 | 0 | 0 |
| etcd | follower | rep2 | n3 | kill n1 | after | 60 | n3 | 130836 | 0 | 32776 | 0 | 0 |
| etcd | follower | rep3 | n2 | kill n1 | before | 30 | n1 | 61618 | 1 | 15589 | 1 | 1 |
| etcd | follower | rep3 | n2 | kill n1 | before | 30 | n2 | 69314 | 0 | 17513 | 0 | 0 |
| etcd | follower | rep3 | n2 | kill n1 | before | 30 | n3 | 69321 | 0 | 17509 | 0 | 0 |
| etcd | follower | rep3 | n2 | kill n1 | during | 60 | n1 | 2430 | 19 | 572 | 1 | 39 |
| etcd | follower | rep3 | n2 | kill n1 | during | 60 | n2 | 225859 | 0 | 56270 | 0 | 0 |
| etcd | follower | rep3 | n2 | kill n1 | during | 60 | n3 | 225864 | 0 | 56275 | 0 | 0 |
| etcd | follower | rep3 | n2 | kill n1 | after | 59 | n1 | 126889 | 0 | 31889 | 0 | 0 |
| etcd | follower | rep3 | n2 | kill n1 | after | 59 | n2 | 126892 | 0 | 31889 | 0 | 0 |
| etcd | follower | rep3 | n2 | kill n1 | after | 59 | n3 | 126888 | 0 | 31885 | 0 | 0 |
| redis | leader | rep1 | n1 | kill n1 | before | 30 | n1 | 0 | 0 | 1793572 | 8 | 0 |
| redis | leader | rep1 | n1 | kill n1 | before | 30 | n2 | 3587458 | 0 | 0 | 0 | 0 |
| redis | leader | rep1 | n1 | kill n1 | before | 30 | n3 | 3587458 | 0 | 0 | 0 | 0 |
| redis | leader | rep1 | n1 | kill n1 | during | 60 | n1 | 0 | 0 | 11374 | 38045 | 4 |
| redis | leader | rep1 | n1 | kill n1 | during | 60 | n2 | 98807 | 0 | 0 | 0 | 4 |
| redis | leader | rep1 | n1 | kill n1 | during | 60 | n3 | 98808 | 0 | 0 | 0 | 4 |
| redis | leader | rep1 | n1 | kill n1 | after | 59 | n1 | 0 | 0 | 3982359 | 0 | 0 |
| redis | leader | rep1 | n1 | kill n1 | after | 59 | n2 | 7847559 | 8 | 0 | 0 | 0 |
| redis | leader | rep1 | n1 | kill n1 | after | 59 | n3 | 8086224 | 4 | 0 | 0 | 0 |
| redis | leader | rep2 | n1 | kill n1 | before | 31 | n1 | 0 | 0 | 1760875 | 8 | 1 |
| redis | leader | rep2 | n1 | kill n1 | before | 31 | n2 | 3521758 | 0 | 0 | 0 | 1 |
| redis | leader | rep2 | n1 | kill n1 | before | 31 | n3 | 3521760 | 0 | 0 | 0 | 1 |
| redis | leader | rep2 | n1 | kill n1 | during | 60 | n1 | 0 | 0 | 96591 | 38097 | 3 |
| redis | leader | rep2 | n1 | kill n1 | during | 60 | n2 | 270319 | 0 | 0 | 0 | 3 |
| redis | leader | rep2 | n1 | kill n1 | during | 60 | n3 | 270318 | 0 | 0 | 0 | 3 |
| redis | leader | rep2 | n1 | kill n1 | after | 58 | n1 | 0 | 0 | 3717168 | 0 | 0 |
| redis | leader | rep2 | n1 | kill n1 | after | 58 | n2 | 7552162 | 3 | 0 | 0 | 0 |
| redis | leader | rep2 | n1 | kill n1 | after | 58 | n3 | 7318037 | 8 | 0 | 0 | 0 |
| redis | leader | rep3 | n1 | kill n1 | before | 30 | n1 | 0 | 0 | 1803491 | 8 | 0 |
| redis | leader | rep3 | n1 | kill n1 | before | 30 | n2 | 3607479 | 0 | 0 | 0 | 0 |
| redis | leader | rep3 | n1 | kill n1 | before | 30 | n3 | 3607477 | 0 | 0 | 0 | 0 |
| redis | leader | rep3 | n1 | kill n1 | during | 60 | n1 | 0 | 0 | 0 | 38198 | 4 |
| redis | leader | rep3 | n1 | kill n1 | during | 60 | n2 | 76886 | 0 | 0 | 0 | 4 |
| redis | leader | rep3 | n1 | kill n1 | during | 60 | n3 | 76886 | 0 | 0 | 0 | 4 |
| redis | leader | rep3 | n1 | kill n1 | after | 59 | n1 | 0 | 0 | 3797520 | 56 | 0 |
| redis | leader | rep3 | n1 | kill n1 | after | 59 | n2 | 7705937 | 4 | 0 | 0 | 0 |
| redis | leader | rep3 | n1 | kill n1 | after | 59 | n3 | 7485480 | 8 | 0 | 0 | 0 |
| redis | follower | rep1 | n1 | kill n2 | before | 30 | n1 | 0 | 0 | 1875173 | 0 | 0 |
| redis | follower | rep1 | n1 | kill n2 | before | 30 | n2 | 3645753 | 8 | 0 | 0 | 0 |
| redis | follower | rep1 | n1 | kill n2 | before | 30 | n3 | 3856122 | 0 | 0 | 0 | 0 |
| redis | follower | rep1 | n1 | kill n2 | during | 60 | n1 | 0 | 0 | 3460571 | 0 | 0 |
| redis | follower | rep1 | n1 | kill n2 | during | 60 | n2 | 0 | 38 | 0 | 0 | 22 |
| redis | follower | rep1 | n1 | kill n2 | during | 60 | n3 | 13839824 | 0 | 0 | 0 | 0 |
| redis | follower | rep1 | n1 | kill n2 | after | 59 | n1 | 0 | 0 | 3691186 | 0 | 0 |
| redis | follower | rep1 | n1 | kill n2 | after | 59 | n2 | 7154132 | 9 | 0 | 0 | 0 |
| redis | follower | rep1 | n1 | kill n2 | after | 59 | n3 | 7616479 | 0 | 0 | 0 | 0 |
| redis | follower | rep2 | n1 | kill n2 | before | 30 | n1 | 0 | 0 | 1867400 | 0 | 0 |
| redis | follower | rep2 | n1 | kill n2 | before | 30 | n2 | 3630320 | 8 | 0 | 0 | 0 |
| redis | follower | rep2 | n1 | kill n2 | before | 30 | n3 | 3841621 | 0 | 0 | 0 | 0 |
| redis | follower | rep2 | n1 | kill n2 | during | 60 | n1 | 0 | 0 | 3422738 | 0 | 0 |
| redis | follower | rep2 | n1 | kill n2 | during | 60 | n2 | 0 | 38 | 0 | 0 | 22 |
| redis | follower | rep2 | n1 | kill n2 | during | 60 | n3 | 13687476 | 0 | 0 | 0 | 0 |
| redis | follower | rep2 | n1 | kill n2 | after | 59 | n1 | 0 | 0 | 3671973 | 0 | 0 |
| redis | follower | rep2 | n1 | kill n2 | after | 59 | n2 | 7233474 | 8 | 0 | 0 | 0 |
| redis | follower | rep2 | n1 | kill n2 | after | 59 | n3 | 7460373 | 0 | 0 | 0 | 0 |
| redis | follower | rep3 | n1 | kill n2 | before | 29 | n1 | 0 | 0 | 1793166 | 0 | 0 |
| redis | follower | rep3 | n1 | kill n2 | before | 29 | n2 | 3480609 | 8 | 0 | 0 | 0 |
| redis | follower | rep3 | n1 | kill n2 | before | 29 | n3 | 3692662 | 0 | 0 | 0 | 0 |
| redis | follower | rep3 | n1 | kill n2 | during | 60 | n1 | 0 | 0 | 3356337 | 0 | 0 |
| redis | follower | rep3 | n1 | kill n2 | during | 60 | n2 | 0 | 38 | 0 | 0 | 22 |
| redis | follower | rep3 | n1 | kill n2 | during | 60 | n3 | 13425734 | 0 | 0 | 0 | 0 |
| redis | follower | rep3 | n1 | kill n2 | after | 60 | n1 | 0 | 0 | 3723522 | 0 | 0 |
| redis | follower | rep3 | n1 | kill n2 | after | 60 | n2 | 7338095 | 8 | 0 | 0 | 0 |
| redis | follower | rep3 | n1 | kill n2 | after | 60 | n3 | 7560984 | 0 | 0 | 0 | 0 |

written: results/e6/e6.csv

written: results/e6/e6-phases.csv

written: results/e6/e6-timeline.csv

