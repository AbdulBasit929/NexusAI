# Verified question bank: the Orion Lahore reference case

484 questions, each with the answer computed by plain SQL on the stored rows (never through the engine) and the live engine's answer scored against it. Synthetic data only.

| Family | Questions | Correct | Wrong | Declined or abstained | Not run |
|---|---|---|---|---|---|
| Calls of one subscriber | 149 | 149 | 0 | 0 | 0 |
| Text messages of one subscriber | 30 | 28 | 0 | 2 | 0 |
| A subscriber and one other number | 32 | 31 | 1 | 0 | 0 |
| Internet sessions | 31 | 31 | 0 | 0 | 0 |
| App usage | 6 | 6 | 0 | 0 | 0 |
| Bank and wallet accounts | 124 | 119 | 0 | 5 | 0 |
| Vehicle plates and cameras | 51 | 50 | 1 | 0 | 0 |
| Handsets | 24 | 24 | 0 | 0 | 0 |
| Cell sites | 16 | 16 | 0 | 0 | 0 |
| Registrations | 16 | 16 | 0 | 0 | 0 |
| The evidence files | 5 | 3 | 2 | 0 | 0 |
| **All** | **484** | **473** | **4** | **7** | **0** |

## Calls of one subscriber

How many voice calls, how long, when, to whom and on which day: counts of calls made and received, talk time, the longest and the average call, calls at night, calls over ten minutes, the first and last event, the number called most, the busiest day, how many different numbers were contacted.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-CALL-001 | How many voice calls does 923000001005 have in this case? | 694 | correct: N: 694. \| first row: n 694 |
| VB-CALL-002 | How many outgoing voice calls did 923000001005 make? | 520 | correct: Outgoing voice calls: 520. \| first row: outgoing_voice_calls 520 |
| VB-CALL-003 | How many incoming voice calls did 923000001005 receive? | 174 | correct: N: 174. \| first row: n 174 |
| VB-CALL-004 | What is the total talk time of 923000001005 across all its voice calls, in seconds? | 69,639 seconds | correct: Total talk time: 69,639. \| first row: total_talk_time 69639 |
| VB-CALL-005 | What is the duration of the longest voice call of 923000001005, in seconds? | 1,272 seconds | correct: Longest duration: 1,272. \| first row: longest_duration 1272 |
| VB-CALL-006 | What is the average duration of the voice calls of 923000001005, in seconds? | 100.34 seconds | correct: Average duration: 100.3444. \| first row: average_duration 100.34438 |
| VB-CALL-007 | How many voice calls of 923000001005 lasted more than 10 minutes? | 4 | correct: Call count: 4. \| first row: call_count 4 |
| VB-CALL-008 | How many voice calls of 923000001005 started between 00:00 and 04:59 Karachi time? | 55 | correct: N: 55. \| first row: n 55 |
| VB-CALL-009 | When was the first call or message of 923000001005? | 2026-02-02 00:29:05 | correct: First event: 2026-02-02 00:29:05. \| first row: first_event 2026-02-02 00:29:05 |
| VB-CALL-010 | When was the last call or message of 923000001005? | 2026-04-17 22:59:16 | correct: Last call: 2026-04-17 22:59:16. \| first row: last_call 2026-04-17 22:59:16 |
| VB-CALL-011 | Which number did 923000001005 call the most with outgoing voice calls? | 923000001036 (before 923000001093, 923000001074, 923000001050) - 105 | correct: 5 rows. \| first row: calls 105; number 923000001036 |
| VB-CALL-012 | Which day had the most voice calls for 923000001005? | 2026-02-17 (before 2026-04-01, 2026-04-02) - 25 | correct: 1 point. \| first row: t 2026-02-17 00:00:00; value 25 |
| VB-CALL-013 | How many different numbers did 923000001005 contact, leaving out services? | 11 | correct: Count of numbers: 11. \| first row: count_of_numbers 11 |
| VB-CALL-014 | How many voice calls does 923000001002 have in this case? | 473 | correct: N: 473. \| first row: n 473 |
| VB-CALL-015 | How many outgoing voice calls did 923000001002 make? | 219 | correct: Outgoing voice calls: 219. \| first row: outgoing_voice_calls 219 |
| VB-CALL-016 | How many incoming voice calls did 923000001002 receive? | 253 | correct: N: 253. \| first row: n 253 |
| VB-CALL-017 | What is the total talk time of 923000001002 across all its voice calls, in seconds? | 57,050 seconds | correct: Total talk time: 57,050. \| first row: total_talk_time 57050 |
| VB-CALL-018 | What is the duration of the longest voice call of 923000001002, in seconds? | 784 seconds | correct: Longest duration: 784. \| first row: longest_duration 784 |
| VB-CALL-019 | What is the average duration of the voice calls of 923000001002, in seconds? | 120.61 seconds | correct: Average duration: 120.6131. \| first row: average_duration 120.613108 |
| VB-CALL-020 | How many voice calls of 923000001002 lasted more than 10 minutes? | 3 | correct: Call count: 3. \| first row: call_count 3 |
| VB-CALL-021 | How many voice calls of 923000001002 started between 00:00 and 04:59 Karachi time? | 17 | correct: N: 17. \| first row: n 17 |
| VB-CALL-022 | When was the first call or message of 923000001002? | 2026-02-02 06:05:32 | correct: First event: 2026-02-02 06:05:32. \| first row: first_event 2026-02-02 06:05:32 |
| VB-CALL-023 | When was the last call or message of 923000001002? | 2026-04-17 21:45:15 | correct: Last call: 2026-04-17 21:45:15. \| first row: last_call 2026-04-17 21:45:15 |
| VB-CALL-024 | Which number did 923000001002 call the most with outgoing voice calls? | 923000001012 (before 923000001054, 923000001048, 923000001032) - 47 | correct: 5 rows. \| first row: calls 47; number 923000001012 |
| VB-CALL-025 | How many different numbers did 923000001002 contact, leaving out services? | 20 | correct: Count of numbers: 20. \| first row: count_of_numbers 20 |
| VB-CALL-026 | How many voice calls does 923000001006 have in this case? | 437 | correct: N: 437. \| first row: n 437 |
| VB-CALL-027 | How many outgoing voice calls did 923000001006 make? | 238 | correct: Outgoing voice calls: 238. \| first row: outgoing_voice_calls 238 |
| VB-CALL-028 | How many incoming voice calls did 923000001006 receive? | 197 | correct: N: 197. \| first row: n 197 |
| VB-CALL-029 | What is the total talk time of 923000001006 across all its voice calls, in seconds? | 47,201 seconds | correct: Total talk time: 47,201. \| first row: total_talk_time 47201 |
| VB-CALL-030 | What is the duration of the longest voice call of 923000001006, in seconds? | 548 seconds | correct: Longest duration: 548. \| first row: longest_duration 548 |
| VB-CALL-031 | What is the average duration of the voice calls of 923000001006, in seconds? | 108.01 seconds | correct: Average duration: 108.0114. \| first row: average_duration 108.011442 |
| VB-CALL-032 | How many voice calls of 923000001006 lasted more than 10 minutes? | 0 | correct: Call count: 0. \| first row: call_count 0 |
| VB-CALL-033 | How many voice calls of 923000001006 started between 00:00 and 04:59 Karachi time? | 46 | correct: N: 46. \| first row: n 46 |
| VB-CALL-034 | When was the first call or message of 923000001006? | 2026-02-02 07:28:45 | correct: First event: 2026-02-02 07:28:45. \| first row: first_event 2026-02-02 07:28:45 |
| VB-CALL-035 | When was the last call or message of 923000001006? | 2026-04-17 23:52:11 | correct: Last call: 2026-04-17 23:52:11. \| first row: last_call 2026-04-17 23:52:11 |
| VB-CALL-036 | Which number did 923000001006 call the most with outgoing voice calls? | 923000001038 (before 923000001001, 923000001021, 923000001026) - 32 | correct: 5 rows. \| first row: calls 32; number 923000001038 |
| VB-CALL-037 | How many different numbers did 923000001006 contact, leaving out services? | 20 | correct: Count of numbers: 20. \| first row: count_of_numbers 20 |
| VB-CALL-038 | How many voice calls does 923000001010 have in this case? | 372 | correct: N: 372. \| first row: n 372 |
| VB-CALL-039 | How many outgoing voice calls did 923000001010 make? | 215 | correct: Outgoing voice calls: 215. \| first row: outgoing_voice_calls 215 |
| VB-CALL-040 | How many incoming voice calls did 923000001010 receive? | 155 | correct: N: 155. \| first row: n 155 |
| VB-CALL-041 | What is the total talk time of 923000001010 across all its voice calls, in seconds? | 41,672 seconds | correct: Total talk time: 41,672. \| first row: total_talk_time 41672 |
| VB-CALL-042 | What is the duration of the longest voice call of 923000001010, in seconds? | 987 seconds | correct: Duration: 987. \| first row: duration 987 |
| VB-CALL-043 | What is the average duration of the voice calls of 923000001010, in seconds? | 112.02 seconds | correct: Average duration: 112.0215. \| first row: average_duration 112.021505 |
| VB-CALL-044 | How many voice calls of 923000001010 lasted more than 10 minutes? | 1 | correct: Call count: 1. \| first row: call_count 1 |
| VB-CALL-045 | How many voice calls of 923000001010 started between 00:00 and 04:59 Karachi time? | 10 | correct: N: 10. \| first row: n 10 |
| VB-CALL-046 | When was the first call or message of 923000001010? | 2026-02-02 08:16:18 | correct: First event: 2026-02-02 08:16:18. \| first row: first_event 2026-02-02 08:16:18 |
| VB-CALL-047 | When was the last call or message of 923000001010? | 2026-04-17 17:30:21 | correct: Last call: 2026-04-17 17:30:21. \| first row: last_call 2026-04-17 17:30:21 |
| VB-CALL-048 | Which number did 923000001010 call the most with outgoing voice calls? | 923000001080 (before 923000001052, 923000001093, 923000001097) - 56 | correct: 5 rows. \| first row: calls 56; number 923000001080 |
| VB-CALL-049 | How many different numbers did 923000001010 contact, leaving out services? | 15 | correct: Count of numbers: 15. \| first row: count_of_numbers 15 |
| VB-CALL-050 | How many voice calls does 923000001000 have in this case? | 261 | correct: N: 261. \| first row: n 261 |
| VB-CALL-051 | How many outgoing voice calls did 923000001000 make? | 107 | correct: Outgoing voice calls: 107. \| first row: outgoing_voice_calls 107 |
| VB-CALL-052 | How many incoming voice calls did 923000001000 receive? | 154 | correct: N: 154. \| first row: n 154 |
| VB-CALL-053 | What is the total talk time of 923000001000 across all its voice calls, in seconds? | 27,465 seconds | correct: Total talk time: 27,465. \| first row: total_talk_time 27465 |
| VB-CALL-054 | What is the duration of the longest voice call of 923000001000, in seconds? | 1,102 seconds | correct: Duration: 1,102. \| first row: duration 1102 |
| VB-CALL-055 | What is the average duration of the voice calls of 923000001000, in seconds? | 105.23 seconds | correct: Average duration: 105.2299. \| first row: average_duration 105.229885 |
| VB-CALL-056 | How many voice calls of 923000001000 lasted more than 10 minutes? | 3 | correct: Count calls: 3. \| first row: count_calls 3 |
| VB-CALL-057 | How many voice calls of 923000001000 started between 00:00 and 04:59 Karachi time? | 6 | correct: N: 6. \| first row: n 6 |
| VB-CALL-058 | When was the first call or message of 923000001000? | 2026-02-02 07:38:45 | correct: First event: 2026-02-02 07:38:45. \| first row: first_event 2026-02-02 07:38:45 |
| VB-CALL-059 | When was the last call or message of 923000001000? | 2026-03-19 21:34:00 | correct: Last call: 2026-03-19 21:34:00. \| first row: last_call 2026-03-19 21:34:00 |
| VB-CALL-060 | Which number did 923000001000 call the most with outgoing voice calls? | 923000001070 (before 923000001015, 923000001089, 923000001098) - 19 | correct: 5 rows. \| first row: calls 19; number 923000001070 |
| VB-CALL-061 | Which day had the most voice calls for 923000001000? | 2026-03-19 (before 2026-02-13, 2026-03-04) - 14 | correct: 1 point. \| first row: t 2026-03-19 00:00:00; value 14 |
| VB-CALL-062 | How many different numbers did 923000001000 contact, leaving out services? | 10 | correct: Count of numbers: 10. \| first row: count_of_numbers 10 |
| VB-CALL-063 | How many voice calls does 923000001007 have in this case? | 253 | correct: N: 253. \| first row: n 253 |
| VB-CALL-064 | How many outgoing voice calls did 923000001007 make? | 119 | correct: Outgoing voice calls: 119. \| first row: outgoing_voice_calls 119 |
| VB-CALL-065 | How many incoming voice calls did 923000001007 receive? | 131 | correct: N: 131. \| first row: n 131 |
| VB-CALL-066 | What is the total talk time of 923000001007 across all its voice calls, in seconds? | 23,487 seconds | correct: Total talk time: 23,487. \| first row: total_talk_time 23487 |
| VB-CALL-067 | What is the duration of the longest voice call of 923000001007, in seconds? | 722 seconds | correct: Longest duration: 722. \| first row: longest_duration 722 |
| VB-CALL-068 | What is the average duration of the voice calls of 923000001007, in seconds? | 92.83 seconds | correct: Average duration: 92.834. \| first row: average_duration 92.833992 |
| VB-CALL-069 | How many voice calls of 923000001007 lasted more than 10 minutes? | 2 | correct: Count calls: 2. \| first row: count_calls 2 |
| VB-CALL-070 | How many voice calls of 923000001007 started between 00:00 and 04:59 Karachi time? | 3 | correct: N: 3. \| first row: n 3 |
| VB-CALL-071 | When was the first call or message of 923000001007? | 2026-02-24 06:31:39 | correct: First event: 2026-02-24 06:31:39. \| first row: first_event 2026-02-24 06:31:39 |
| VB-CALL-072 | When was the last call or message of 923000001007? | 2026-04-17 22:20:58 | correct: Last call: 2026-04-17 22:20:58. \| first row: last_call 2026-04-17 22:20:58 |
| VB-CALL-073 | Which number did 923000001007 call the most with outgoing voice calls? | 923000001057 (before 923000001073, 923000001087, 923000001072) - 29 | correct: 5 rows. \| first row: calls 29; number 923000001057 |
| VB-CALL-074 | Which day had the most voice calls for 923000001007? | 2026-03-18 (before 2026-03-11, 2026-02-24) - 11 | correct: 1 point. \| first row: t 2026-03-18 00:00:00; value 11 |
| VB-CALL-075 | How many different numbers did 923000001007 contact, leaving out services? | 8 | correct: Count of numbers: 8. \| first row: count_of_numbers 8 |
| VB-CALL-076 | How many voice calls does 923000001009 have in this case? | 342 | correct: N: 342. \| first row: n 342 |
| VB-CALL-077 | How many outgoing voice calls did 923000001009 make? | 198 | correct: Outgoing voice calls: 198. \| first row: outgoing_voice_calls 198 |
| VB-CALL-078 | How many incoming voice calls did 923000001009 receive? | 144 | correct: N: 144. \| first row: n 144 |
| VB-CALL-079 | What is the total talk time of 923000001009 across all its voice calls, in seconds? | 39,991 seconds | correct: Total talk time: 39,991. \| first row: total_talk_time 39991 |
| VB-CALL-080 | What is the duration of the longest voice call of 923000001009, in seconds? | 1,800 seconds | correct: Longest duration: 1,800. \| first row: longest_duration 1800 |
| VB-CALL-081 | What is the average duration of the voice calls of 923000001009, in seconds? | 116.93 seconds | correct: Average duration: 116.9327. \| first row: average_duration 116.932749 |
| VB-CALL-082 | How many voice calls of 923000001009 lasted more than 10 minutes? | 3 | correct: Call count: 3. \| first row: call_count 3 |
| VB-CALL-083 | How many voice calls of 923000001009 started between 00:00 and 04:59 Karachi time? | 19 | correct: N: 19. \| first row: n 19 |
| VB-CALL-084 | When was the first call or message of 923000001009? | 2026-02-02 09:25:19 | correct: First event: 2026-02-02 09:25:19. \| first row: first_event 2026-02-02 09:25:19 |
| VB-CALL-085 | When was the last call or message of 923000001009? | 2026-04-17 21:04:48 | correct: Last call: 2026-04-17 21:04:48. \| first row: last_call 2026-04-17 21:04:48 |
| VB-CALL-086 | Which number did 923000001009 call the most with outgoing voice calls? | 923000001053 (before 923000001044, 923000001066, 923000001024) - 38 | correct: 5 rows. \| first row: calls 38; number 923000001053 |
| VB-CALL-087 | How many different numbers did 923000001009 contact, leaving out services? | 16 | correct: Count of numbers: 16. \| first row: count_of_numbers 16 |
| VB-CALL-088 | How many voice calls does 923000001001 have in this case? | 326 | correct: N: 326. \| first row: n 326 |
| VB-CALL-089 | How many outgoing voice calls did 923000001001 make? | 146 | correct: Outgoing voice calls: 146. \| first row: outgoing_voice_calls 146 |
| VB-CALL-090 | How many incoming voice calls did 923000001001 receive? | 180 | correct: N: 180. \| first row: n 180 |
| VB-CALL-091 | What is the total talk time of 923000001001 across all its voice calls, in seconds? | 31,663 seconds | correct: Total talk time: 31,663. \| first row: total_talk_time 31663 |
| VB-CALL-092 | What is the duration of the longest voice call of 923000001001, in seconds? | 733 seconds | correct: Longest duration: 733. \| first row: longest_duration 733 |
| VB-CALL-093 | What is the average duration of the voice calls of 923000001001, in seconds? | 97.13 seconds | correct: Average duration: 97.1258. \| first row: average_duration 97.125767 |
| VB-CALL-094 | How many voice calls of 923000001001 lasted more than 10 minutes? | 1 | correct: Call count: 1. \| first row: call_count 1 |
| VB-CALL-095 | How many voice calls of 923000001001 started between 00:00 and 04:59 Karachi time? | 15 | correct: N: 15. \| first row: n 15 |
| VB-CALL-096 | When was the first call or message of 923000001001? | 2026-02-02 05:23:53 | correct: First event: 2026-02-02 05:23:53. \| first row: first_event 2026-02-02 05:23:53 |
| VB-CALL-097 | When was the last call or message of 923000001001? | 2026-04-17 20:05:51 | correct: Last call: 2026-04-17 20:05:51. \| first row: last_call 2026-04-17 20:05:51 |
| VB-CALL-098 | Which number did 923000001001 call the most with outgoing voice calls? | 923000001078 (before 923000001038, 923000001006, 923000001069) - 33 | correct: 5 rows. \| first row: calls 33; number 923000001078 |
| VB-CALL-099 | Which day had the most voice calls for 923000001001? | 2026-02-05 (before 2026-02-25, 2026-02-16) - 11 | correct: 1 point. \| first row: t 2026-02-05 00:00:00; value 11 |
| VB-CALL-100 | How many different numbers did 923000001001 contact, leaving out services? | 10 | correct: Count of numbers: 10. \| first row: count_of_numbers 10 |
| VB-CALL-101 | How many voice calls does 923000001008 have in this case? | 206 | correct: N: 206. \| first row: n 206 |
| VB-CALL-102 | How many outgoing voice calls did 923000001008 make? | 109 | correct: Outgoing voice calls: 109. \| first row: outgoing_voice_calls 109 |
| VB-CALL-103 | How many incoming voice calls did 923000001008 receive? | 97 | correct: N: 97. \| first row: n 97 |
| VB-CALL-104 | What is the total talk time of 923000001008 across all its voice calls, in seconds? | 18,793 seconds | correct: Total talk time: 18,793. \| first row: total_talk_time 18793 |
| VB-CALL-105 | What is the duration of the longest voice call of 923000001008, in seconds? | 371 seconds | correct: Duration: 371. \| first row: duration 371 |
| VB-CALL-106 | What is the average duration of the voice calls of 923000001008, in seconds? | 91.23 seconds | correct: Average duration: 91.2282. \| first row: average_duration 91.228155 |
| VB-CALL-107 | How many voice calls of 923000001008 lasted more than 10 minutes? | 0 | correct: Call count: 0. \| first row: call_count 0 |
| VB-CALL-108 | How many voice calls of 923000001008 started between 00:00 and 04:59 Karachi time? | 7 | correct: N: 7. \| first row: n 7 |
| VB-CALL-109 | When was the first call or message of 923000001008? | 2026-03-05 10:18:48 | correct: First event: 2026-03-05 10:18:48. \| first row: first_event 2026-03-05 10:18:48 |
| VB-CALL-110 | When was the last call or message of 923000001008? | 2026-04-17 22:50:59 | correct: Last call: 2026-04-17 22:50:59. \| first row: last_call 2026-04-17 22:50:59 |
| VB-CALL-111 | How many different numbers did 923000001008 contact, leaving out services? | 8 | correct: Count of numbers: 8. \| first row: count_of_numbers 8 |
| VB-CALL-112 | How many voice calls does 923000001003 have in this case? | 190 | correct: N: 190. \| first row: n 190 |
| VB-CALL-113 | How many outgoing voice calls did 923000001003 make? | 95 | correct: Outgoing voice calls: 95. \| first row: outgoing_voice_calls 95 |
| VB-CALL-114 | How many incoming voice calls did 923000001003 receive? | 93 | correct: N: 93. \| first row: n 93 |
| VB-CALL-115 | What is the total talk time of 923000001003 across all its voice calls, in seconds? | 19,255 seconds | correct: Total talk time: 19,255. \| first row: total_talk_time 19255 |
| VB-CALL-116 | What is the duration of the longest voice call of 923000001003, in seconds? | 775 seconds | correct: Longest duration: 775. \| first row: longest_duration 775 |
| VB-CALL-117 | What is the average duration of the voice calls of 923000001003, in seconds? | 101.34 seconds | correct: Average duration: 101.3421. \| first row: average_duration 101.342105 |
| VB-CALL-118 | How many voice calls of 923000001003 lasted more than 10 minutes? | 1 | correct: Call count: 1. \| first row: call_count 1 |
| VB-CALL-119 | How many voice calls of 923000001003 started between 00:00 and 04:59 Karachi time? | 9 | correct: N: 9. \| first row: n 9 |
| VB-CALL-120 | When was the first call or message of 923000001003? | 2026-03-13 04:44:09 | correct: First event: 2026-03-13 04:44:09. \| first row: first_event 2026-03-13 04:44:09 |
| VB-CALL-121 | When was the last call or message of 923000001003? | 2026-04-17 21:33:43 | correct: Last call: 2026-04-17 21:33:43. \| first row: last_call 2026-04-17 21:33:43 |
| VB-CALL-122 | Which number did 923000001003 call the most with outgoing voice calls? | 923000001069 (before 923000001021, 923000001095, 923000001006) - 18 | correct: 5 rows. \| first row: calls 18; number 923000001069 |
| VB-CALL-123 | Which day had the most voice calls for 923000001003? | 2026-04-10 (before 2026-03-19, 2026-03-30) - 10 | correct: 1 point. \| first row: t 2026-04-10 00:00:00; value 10 |
| VB-CALL-124 | How many different numbers did 923000001003 contact, leaving out services? | 8 | correct: Count of numbers: 8. \| first row: count_of_numbers 8 |
| VB-CALL-125 | How many voice calls does 923000001011 have in this case? | 169 | correct: N: 169. \| first row: n 169 |
| VB-CALL-126 | How many outgoing voice calls did 923000001011 make? | 92 | correct: Outgoing voice calls: 92. \| first row: outgoing_voice_calls 92 |
| VB-CALL-127 | How many incoming voice calls did 923000001011 receive? | 76 | correct: N: 76. \| first row: n 76 |
| VB-CALL-128 | What is the total talk time of 923000001011 across all its voice calls, in seconds? | 17,913 seconds | correct: Total talk time: 17,913. \| first row: total_talk_time 17913 |
| VB-CALL-129 | What is the duration of the longest voice call of 923000001011, in seconds? | 662 seconds | correct: Duration: 662. \| first row: duration 662 |
| VB-CALL-130 | What is the average duration of the voice calls of 923000001011, in seconds? | 105.99 seconds | correct: Average duration: 105.9941. \| first row: average_duration 105.994083 |
| VB-CALL-131 | How many voice calls of 923000001011 lasted more than 10 minutes? | 1 | correct: Count calls: 1. \| first row: count_calls 1 |
| VB-CALL-132 | How many voice calls of 923000001011 started between 00:00 and 04:59 Karachi time? | 4 | correct: N: 4. \| first row: n 4 |
| VB-CALL-133 | When was the first call or message of 923000001011? | 2026-02-02 02:20:13 | correct: First event: 2026-02-02 02:20:13. \| first row: first_event 2026-02-02 02:20:13 |
| VB-CALL-134 | When was the last call or message of 923000001011? | 2026-03-09 19:53:38 | correct: Last call: 2026-03-09 19:53:38. \| first row: last_call 2026-03-09 19:53:38 |
| VB-CALL-135 | Which number did 923000001011 call the most with outgoing voice calls? | 923000001021 (before 923000001069, 923000001001, 923000001006) - 17 | correct: 5 rows. \| first row: calls 17; number 923000001021 |
| VB-CALL-136 | How many different numbers did 923000001011 contact, leaving out services? | 8 | correct: Count of numbers: 8. \| first row: count_of_numbers 8 |
| VB-CALL-137 | How many voice calls does 923000001004 have in this case? | 148 | correct: N: 148. \| first row: n 148 |
| VB-CALL-138 | How many outgoing voice calls did 923000001004 make? | 82 | correct: Outgoing voice calls: 82. \| first row: outgoing_voice_calls 82 |
| VB-CALL-139 | How many incoming voice calls did 923000001004 receive? | 66 | correct: N: 66. \| first row: n 66 |
| VB-CALL-140 | What is the total talk time of 923000001004 across all its voice calls, in seconds? | 13,707 seconds | correct: Total talk time: 13,707. \| first row: total_talk_time 13707 |
| VB-CALL-141 | What is the duration of the longest voice call of 923000001004, in seconds? | 545 seconds | correct: Longest duration: 545. \| first row: longest_duration 545 |
| VB-CALL-142 | What is the average duration of the voice calls of 923000001004, in seconds? | 92.61 seconds | correct: Average duration: 92.6149. \| first row: average_duration 92.614865 |
| VB-CALL-143 | How many voice calls of 923000001004 lasted more than 10 minutes? | 0 | correct: Count calls: 0. \| first row: count_calls 0 |
| VB-CALL-144 | How many voice calls of 923000001004 started between 00:00 and 04:59 Karachi time? | 6 | correct: N: 6. \| first row: n 6 |
| VB-CALL-145 | When was the first call or message of 923000001004? | 2026-02-02 07:37:42 | correct: First event: 2026-02-02 07:37:42. \| first row: first_event 2026-02-02 07:37:42 |
| VB-CALL-146 | When was the last call or message of 923000001004? | 2026-03-03 19:58:06 | correct: Last call: 2026-03-03 19:58:06. \| first row: last_call 2026-03-03 19:58:06 |
| VB-CALL-147 | Which number did 923000001004 call the most with outgoing voice calls? | 923000001066 (before 923000001044, 923000001061, 923000001090) - 24 | correct: 5 rows. \| first row: calls 24; number 923000001066 |
| VB-CALL-148 | Which day had the most voice calls for 923000001004? | 2026-02-12 (before 2026-02-16, 2026-02-03) - 9 | correct: 1 point. \| first row: t 2026-02-12 00:00:00; value 9 |
| VB-CALL-149 | How many different numbers did 923000001004 contact, leaving out services? | 8 | correct: Count of numbers: 8. \| first row: count_of_numbers 8 |

## Text messages of one subscriber

How many text messages were sent and received, to whom most, and when the first was sent.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-MESS-001 | How many text messages does 923000001002 have in this case? | 213 | correct: N: 213. \| first row: n 213 |
| VB-MESS-002 | How many text messages did 923000001002 send? | 93 | correct: N: 93. \| first row: n 93 |
| VB-MESS-003 | How many text messages did 923000001002 receive? | 118 | correct: N: 118. \| first row: n 118 |
| VB-MESS-004 | Which number did 923000001002 send the most text messages to? | 923000001048 (before 923000001012, 923000001054, 923000001027) - 21 | correct: 5 results: 923000001048 (21), 923000001012 (19), 923000001054 (17), 923000001032 (12), 923000001027 (12). \| fi |
| VB-MESS-005 | When was the first text message of 923000001002? | 2026-02-02 06:12:39 | correct: First sms: 2026-02-02 06:12:39. \| first row: first_sms 2026-02-02 06:12:39 |
| VB-MESS-006 | How many text messages does 923000001006 have in this case? | 203 | correct: N: 203. \| first row: n 203 |
| VB-MESS-007 | How many text messages did 923000001006 send? | 103 | correct: N: 103. \| first row: n 103 |
| VB-MESS-008 | How many text messages did 923000001006 receive? | 99 | correct: N: 99. \| first row: n 99 |
| VB-MESS-009 | Which number did 923000001006 send the most text messages to? | 923000001038 (before 923000001031, 923000001003, 923000001021) - 16 | abstained: I did not give an answer, because I could not show that the query I wrote asks everything you asked: the quest |
| VB-MESS-010 | When was the first text message of 923000001006? | 2026-02-02 12:40:07 | correct: First sms: 2026-02-02 12:40:07. \| first row: first_sms 2026-02-02 12:40:07 |
| VB-MESS-011 | How many text messages does 923000001010 have in this case? | 172 | correct: N: 172. \| first row: n 172 |
| VB-MESS-012 | How many text messages did 923000001010 send? | 97 | correct: N: 97. \| first row: n 97 |
| VB-MESS-013 | How many text messages did 923000001010 receive? | 74 | correct: N: 74. \| first row: n 74 |
| VB-MESS-014 | Which number did 923000001010 send the most text messages to? | 923000001052 (before 923000001093, 923000001080, 923000001074) - 22 | correct: 5 results: 923000001052 (22), 923000001093 (21), 923000001080 (20), 923000001074 (16), 923000001097 (14). \| fi |
| VB-MESS-015 | When was the first text message of 923000001010? | 2026-02-02 12:34:45 | correct: First sms: 2026-02-02 12:34:45. \| first row: first_sms 2026-02-02 12:34:45 |
| VB-MESS-016 | How many text messages does 923000001000 have in this case? | 138 | correct: N: 138. \| first row: n 138 |
| VB-MESS-017 | How many text messages did 923000001000 send? | 52 | correct: N: 52. \| first row: n 52 |
| VB-MESS-018 | How many text messages did 923000001000 receive? | 86 | correct: N: 86. \| first row: n 86 |
| VB-MESS-019 | Which number did 923000001000 send the most text messages to? | 923000001015 (before 923000001089, 923000001080, 923000001098) - 11 | abstained: I did not give an answer, because I could not show that the query I wrote asks everything you asked: the quest |
| VB-MESS-020 | When was the first text message of 923000001000? | 2026-02-02 07:38:45 | correct: First sms: 2026-02-02 07:38:45. \| first row: first_sms 2026-02-02 07:38:45 |
| VB-MESS-021 | How many text messages does 923000001003 have in this case? | 110 | correct: N: 110. \| first row: n 110 |
| VB-MESS-022 | How many text messages did 923000001003 send? | 56 | correct: N: 56. \| first row: n 56 |
| VB-MESS-023 | How many text messages did 923000001003 receive? | 53 | correct: N: 53. \| first row: n 53 |
| VB-MESS-024 | Which number did 923000001003 send the most text messages to? | 923000001069 (before 923000001006, 923000001031, 923000001001) - 14 | correct: 5 results: 923000001069 (14), 923000001006 (9), 923000001031 (9), 923000001021 (7), 923000001095 (7). \| first  |
| VB-MESS-025 | When was the first text message of 923000001003? | 2026-03-13 07:52:14 | correct: First sms: 2026-03-13 07:52:14. \| first row: first_sms 2026-03-13 07:52:14 |
| VB-MESS-026 | How many text messages does 923000001007 have in this case? | 108 | correct: N: 108. \| first row: n 108 |
| VB-MESS-027 | How many text messages did 923000001007 send? | 61 | correct: N: 61. \| first row: n 61 |
| VB-MESS-028 | How many text messages did 923000001007 receive? | 47 | correct: N: 47. \| first row: n 47 |
| VB-MESS-029 | Which number did 923000001007 send the most text messages to? | 923000001057 (before 923000001017, 923000001087, 923000001054) - 17 | correct: 5 results: 923000001057 (17), 923000001017 (10), 923000001087 (10), 923000001054 (7), 923000001086 (6). \| firs |
| VB-MESS-030 | When was the first text message of 923000001007? | 2026-02-24 06:31:39 | correct: First sms: 2026-02-24 06:31:39. \| first row: first_sms 2026-02-24 06:31:39 |

## A subscriber and one other number

How many calls and messages two numbers had, how many outgoing calls, the total duration, and when they last had contact.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-PAIR-001 | How many calls and messages did 923000001005 have with 923000001036? | 150 | correct: N: 150. \| first row: n 150 |
| VB-PAIR-002 | How many outgoing voice calls did 923000001005 make to 923000001036? | 105 | correct: Call count: 105. \| first row: call_count 105 |
| VB-PAIR-003 | What is the total duration of the voice calls between 923000001005 and 923000001036, in seconds? | 14,673 seconds | correct: 2 rows. \| first row: total_duration None |
| VB-PAIR-004 | When was the last contact between 923000001005 and 923000001036? | 2026-04-17 19:37:12 | correct: Last contact: 2026-04-17 19:37:12 PKT. \| first row: last_contact 2026-04-17 19:37:12 PKT |
| VB-PAIR-005 | How many calls and messages did 923000001002 have with 923000001027? | 146 | correct: N: 146. \| first row: n 146 |
| VB-PAIR-006 | How many outgoing voice calls did 923000001002 make to 923000001027? | 25 | correct: Call count: 25. \| first row: call_count 25 |
| VB-PAIR-007 | What is the total duration of the voice calls between 923000001002 and 923000001027, in seconds? | 12,290 seconds | wrong: Total duration: 57,670. \| first row: total_duration 57670 |
| VB-PAIR-008 | When was the last contact between 923000001002 and 923000001027? | 2026-04-17 17:15:59 | correct: Last contact: 2026-04-17 17:15:59 PKT. \| first row: last_contact 2026-04-17 17:15:59 PKT |
| VB-PAIR-009 | How many calls and messages did 923000001006 have with 923000001038? | 93 | correct: N: 93. \| first row: n 93 |
| VB-PAIR-010 | How many outgoing voice calls did 923000001006 make to 923000001038? | 32 | correct: Call count: 32. \| first row: call_count 32 |
| VB-PAIR-011 | What is the total duration of the voice calls between 923000001006 and 923000001038, in seconds? | 6,183 seconds | correct: 2 rows. \| first row: total_duration None |
| VB-PAIR-012 | When was the last contact between 923000001006 and 923000001038? | 2026-04-17 16:58:18 | correct: Last contact: 2026-04-17 16:58:18 PKT. \| first row: last_contact 2026-04-17 16:58:18 PKT |
| VB-PAIR-013 | How many calls and messages did 923000001010 have with 923000001080? | 124 | correct: N: 124. \| first row: n 124 |
| VB-PAIR-014 | How many outgoing voice calls did 923000001010 make to 923000001080? | 56 | correct: Call count: 56. \| first row: call_count 56 |
| VB-PAIR-015 | What is the total duration of the voice calls between 923000001010 and 923000001080, in seconds? | 8,512 seconds | correct: 2 rows. \| first row: total_duration None |
| VB-PAIR-016 | When was the last contact between 923000001010 and 923000001080? | 2026-04-16 20:42:35 | correct: Last contact: 2026-04-16 20:42:35 PKT. \| first row: last_contact 2026-04-16 20:42:35 PKT |
| VB-PAIR-017 | How many calls and messages did 923000001000 have with 923000001098? | 75 | correct: N: 75. \| first row: n 75 |
| VB-PAIR-018 | How many outgoing voice calls did 923000001000 make to 923000001098? | 14 | correct: Call count: 14. \| first row: call_count 14 |
| VB-PAIR-019 | What is the total duration of the voice calls between 923000001000 and 923000001098, in seconds? | 5,613 seconds | correct: 2 rows. \| first row: total_duration None |
| VB-PAIR-020 | When was the last contact between 923000001000 and 923000001098? | 2026-03-19 21:34:00 | correct: Last contact: 2026-03-19 21:34:00 PKT. \| first row: last_contact 2026-03-19 21:34:00 PKT |
| VB-PAIR-021 | How many calls and messages did 923000001007 have with 923000001057? | 84 | correct: N: 84. \| first row: n 84 |
| VB-PAIR-022 | How many outgoing voice calls did 923000001007 make to 923000001057? | 29 | correct: Call count: 29. \| first row: call_count 29 |
| VB-PAIR-023 | What is the total duration of the voice calls between 923000001007 and 923000001057, in seconds? | 5,298 seconds | correct: 2 rows. \| first row: total_duration None |
| VB-PAIR-024 | When was the last contact between 923000001007 and 923000001057? | 2026-04-17 20:26:35 | correct: Last contact: 2026-04-17 20:26:35 PKT. \| first row: last_contact 2026-04-17 20:26:35 PKT |
| VB-PAIR-025 | How many calls and messages did 923000001009 have with 923000001053? | 72 | correct: N: 72. \| first row: n 72 |
| VB-PAIR-026 | How many outgoing voice calls did 923000001009 make to 923000001053? | 38 | correct: Call count: 38. \| first row: call_count 38 |
| VB-PAIR-027 | What is the total duration of the voice calls between 923000001009 and 923000001053, in seconds? | 6,659 seconds | correct: 2 rows. \| first row: total_duration None |
| VB-PAIR-028 | When was the last contact between 923000001009 and 923000001053? | 2026-04-17 21:04:48 | correct: Last contact: 2026-04-17 21:04:48 PKT. \| first row: last_contact 2026-04-17 21:04:48 PKT |
| VB-PAIR-029 | How many calls and messages did 923000001001 have with 923000001078? | 63 | correct: N: 63. \| first row: n 63 |
| VB-PAIR-030 | How many outgoing voice calls did 923000001001 make to 923000001078? | 33 | correct: Call count: 33. \| first row: call_count 33 |
| VB-PAIR-031 | What is the total duration of the voice calls between 923000001001 and 923000001078, in seconds? | 6,546 seconds | correct: 2 rows. \| first row: total_duration None |
| VB-PAIR-032 | When was the last contact between 923000001001 and 923000001078? | 2026-04-15 16:19:31 | correct: Last contact: 2026-04-15 16:19:31 PKT. \| first row: last_contact 2026-04-15 16:19:31 PKT |

## Internet sessions

Sessions, bytes downloaded and uploaded, the number of different apps, the app and the host used most, and which number has the most sessions.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-INTE-001 | How many internet sessions does 923000001010 have? | 431 | correct: Value: 431. \| first row: value 431 |
| VB-INTE-002 | How many bytes did 923000001010 download in total? | 214,448,386 bytes | correct: Value: 214,448,386. \| first row: value 214448386 |
| VB-INTE-003 | How many bytes did 923000001010 upload in total? | 20,985,010 bytes | correct: Value: 20,985,010. \| first row: value 20985010 |
| VB-INTE-004 | How many different apps did 923000001010 use in its internet sessions? | 3 | correct: Value: 3. \| first row: value 3 |
| VB-INTE-005 | Which app did 923000001010 use the most in its internet sessions? | app_a (before app_b, app_c) - 220 | correct: 1 result: app_a (106,452,361). \| first row: label app_a; value 106452361 |
| VB-INTE-006 | How many internet sessions does 923000001006 have? | 430 | correct: Value: 430. \| first row: value 430 |
| VB-INTE-007 | How many bytes did 923000001006 download in total? | 208,354,669 bytes | correct: Value: 208,354,669. \| first row: value 208354669 |
| VB-INTE-008 | How many bytes did 923000001006 upload in total? | 21,137,121 bytes | correct: Value: 21,137,121. \| first row: value 21137121 |
| VB-INTE-009 | How many different apps did 923000001006 use in its internet sessions? | 3 | correct: Value: 3. \| first row: value 3 |
| VB-INTE-010 | Which app did 923000001006 use the most in its internet sessions? | app_a (before app_b, app_c) - 228 | correct: 1 result: app_a (112,617,568). \| first row: label app_a; value 112617568 |
| VB-INTE-011 | How many internet sessions does 923000001009 have? | 421 | correct: Value: 421. \| first row: value 421 |
| VB-INTE-012 | How many bytes did 923000001009 download in total? | 200,016,221 bytes | correct: Value: 200,016,221. \| first row: value 200016221 |
| VB-INTE-013 | How many bytes did 923000001009 upload in total? | 20,771,501 bytes | correct: Value: 20,771,501. \| first row: value 20771501 |
| VB-INTE-014 | How many different apps did 923000001009 use in its internet sessions? | 3 | correct: Value: 3. \| first row: value 3 |
| VB-INTE-015 | Which app did 923000001009 use the most in its internet sessions? | app_a (before app_c, app_b) - 213 | correct: 1 result: app_a (100,070,176). \| first row: label app_a; value 100070176 |
| VB-INTE-016 | How many internet sessions does 923000001002 have? | 399 | correct: Value: 399. \| first row: value 399 |
| VB-INTE-017 | How many bytes did 923000001002 download in total? | 194,375,025 bytes | correct: Value: 194,375,025. \| first row: value 194375025 |
| VB-INTE-018 | How many bytes did 923000001002 upload in total? | 20,577,123 bytes | correct: Value: 20,577,123. \| first row: value 20577123 |
| VB-INTE-019 | How many different apps did 923000001002 use in its internet sessions? | 3 | correct: Value: 3. \| first row: value 3 |
| VB-INTE-020 | Which app did 923000001002 use the most in its internet sessions? | app_a (before app_c, app_b) - 191 | correct: 1 result: app_a (94,002,087). \| first row: label app_a; value 94002087 |
| VB-INTE-021 | How many internet sessions does 923000001005 have? | 389 | correct: Value: 389. \| first row: value 389 |
| VB-INTE-022 | How many bytes did 923000001005 download in total? | 195,404,638 bytes | correct: Value: 195,404,638. \| first row: value 195404638 |
| VB-INTE-023 | How many bytes did 923000001005 upload in total? | 19,464,161 bytes | correct: Value: 19,464,161. \| first row: value 19464161 |
| VB-INTE-024 | How many different apps did 923000001005 use in its internet sessions? | 3 | correct: Value: 3. \| first row: value 3 |
| VB-INTE-025 | Which app did 923000001005 use the most in its internet sessions? | app_a (before app_c, app_b) - 192 | correct: 1 result: app_a (101,188,667). \| first row: label app_a; value 101188667 |
| VB-INTE-026 | How many internet sessions does 923000001001 have? | 384 | correct: Value: 384. \| first row: value 384 |
| VB-INTE-027 | How many bytes did 923000001001 download in total? | 196,829,437 bytes | correct: Value: 196,829,437. \| first row: value 196829437 |
| VB-INTE-028 | How many bytes did 923000001001 upload in total? | 20,237,131 bytes | correct: Value: 20,237,131. \| first row: value 20237131 |
| VB-INTE-029 | How many different apps did 923000001001 use in its internet sessions? | 3 | correct: Value: 3. \| first row: value 3 |
| VB-INTE-030 | Which app did 923000001001 use the most in its internet sessions? | app_a (before app_c, app_b) - 207 | correct: 1 result: app_a (108,164,907). \| first row: label app_a; value 108164907 |
| VB-INTE-031 | Which number has the most internet sessions? | 923000001010 | correct: 1 result: 923000001010 (431). \| first row: label 923000001010; value 431 |

## App usage

Records of app use per number, the number of different apps and the one used most.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-APPS-001 | How many app usage records does 923000001001 have? | 390 | correct: Value: 390. \| first row: value 390 |
| VB-APPS-002 | How many different apps did 923000001001 use? | 3 | correct: Value: 3. \| first row: value 3 |
| VB-APPS-003 | Which app did 923000001001 use the most? | app_a (before app_c, app_b) - 210 | correct: 1 result: app_a (210). \| first row: label app_a; value 210 |
| VB-APPS-004 | How many app usage records does 923000001000 have? | 267 | correct: Value: 267. \| first row: value 267 |
| VB-APPS-005 | How many different apps did 923000001000 use? | 3 | correct: Value: 3. \| first row: value 3 |
| VB-APPS-006 | Which app did 923000001000 use the most? | app_a (before app_b, app_c) - 141 | correct: 1 result: app_a (141). \| first row: label app_a; value 141 |

## Bank and wallet accounts

Transactions of an account, the totals credited and debited, the largest debit, the last balance, the first and last day, the account paid the most, how many different accounts were paid, who sent money, debits above an amount.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-BANK-001 | How many transactions does the account PK00SYNT0000000000002010 have? | 53 | correct: N: 53. \| first row: n 53 |
| VB-BANK-002 | What is the total amount credited to the account PK00SYNT0000000000002010? | 1,156,096.41 rupees | correct: Total credited: 1,156,096.41. \| first row: total_credited 1156096.41 |
| VB-BANK-003 | What is the total amount debited from the account PK00SYNT0000000000002010? | 4,161,008.11 rupees | correct: Total debited: 4,161,008.11. \| first row: total_debited 4161008.11 |
| VB-BANK-004 | What is the largest single debit of the account PK00SYNT0000000000002010? | 490,500 rupees | declined:  |
| VB-BANK-005 | What was the balance after the last transaction of the account PK00SYNT0000000000002010? | 3,005,088.30 rupees | correct: Balance: 3,005,088.3. \| first row: balance 3005088.3 |
| VB-BANK-006 | On which day was the first transaction of the account PK00SYNT0000000000002010? | 2026-02-05 | correct: First transaction day: 2026-02-05. \| first row: first_transaction_day 2026-02-05 |
| VB-BANK-007 | On which day was the last transaction of the account PK00SYNT0000000000002010? | 2026-04-16 | correct: Day: 2026-04-16. \| first row: day 2026-04-16 |
| VB-BANK-008 | Who did the account PK00SYNT0000000000002010 pay the most money to? | PK00SYNT0000000000005009 (before PK00SYNT0000000000005002, PK00SYNT0000000000005011) - 1,312,676.90 | correct: Counterparty PK00SYNT0000000000005009; Total amount 1,312,676.9. \| first row: counterparty PK00SYNT00000000000 |
| VB-BANK-009 | How many different accounts or numbers did the account PK00SYNT0000000000002010 pay? | 12 | correct: Count: 12. \| first row: count 12 |
| VB-BANK-010 | How many debits of the account PK00SYNT0000000000002010 were above 245250 rupees? | 5 | correct: N: 5. \| first row: n 5 |
| VB-BANK-011 | How many transactions does the account PK00SYNT0000000000002004 have? | 52 | correct: N: 52. \| first row: n 52 |
| VB-BANK-012 | What is the total amount credited to the account PK00SYNT0000000000002004? | 1,771,898.97 rupees | correct: Total credited: 1,771,898.97. \| first row: total_credited 1771898.97 |
| VB-BANK-013 | What is the total amount debited from the account PK00SYNT0000000000002004? | 4,186,972.88 rupees | correct: Total debited: 4,186,972.88. \| first row: total_debited 4186972.88 |
| VB-BANK-014 | What is the largest single debit of the account PK00SYNT0000000000002004? | 494,500 rupees | correct: Largest: 494,500. \| first row: largest 494500 |
| VB-BANK-015 | What was the balance after the last transaction of the account PK00SYNT0000000000002004? | 3,510,987.64 rupees | correct: Balance: 3,510,987.64. \| first row: balance 3510987.64 |
| VB-BANK-016 | On which day was the first transaction of the account PK00SYNT0000000000002004? | 2026-02-03 | correct: Day: 2026-02-03. \| first row: day 2026-02-03 |
| VB-BANK-017 | On which day was the last transaction of the account PK00SYNT0000000000002004? | 2026-04-17 | correct: Day: 2026-04-17. \| first row: day 2026-04-17 |
| VB-BANK-018 | Who did the account PK00SYNT0000000000002004 pay the most money to? | PK00SYNT0000000000005005 (before PK00SYNT0000000000005010, PK00SYNT0000000000005011) - 614,205.31 | correct: Counterparty PK00SYNT0000000000005005; Transactions 2; Total amount 614,205.31. \| first row: counterparty PK00 |
| VB-BANK-019 | How many different accounts or numbers did the account PK00SYNT0000000000002004 pay? | 12 | correct: Count: 12. \| first row: count 12 |
| VB-BANK-020 | How many debits of the account PK00SYNT0000000000002004 were above 247250 rupees? | 6 | correct: N: 6. \| first row: n 6 |
| VB-BANK-021 | How many transactions does the account PK00SYNT0000000000002003 have? | 46 | correct: N: 46. \| first row: n 46 |
| VB-BANK-022 | What is the total amount credited to the account PK00SYNT0000000000002003? | 1,847,854.55 rupees | correct: Total credited: 1,847,854.55. \| first row: total_credited 1847854.55 |
| VB-BANK-023 | What is the total amount debited from the account PK00SYNT0000000000002003? | 2,646,194.03 rupees | correct: Total debited: 2,646,194.03. \| first row: total_debited 2646194.03 |
| VB-BANK-024 | What is the largest single debit of the account PK00SYNT0000000000002003? | 384,500 rupees | correct: Largest: 384,500. \| first row: largest 384500 |
| VB-BANK-025 | What was the balance after the last transaction of the account PK00SYNT0000000000002003? | 5,221,660.52 rupees | correct: Balance after: 5,221,660.52. \| first row: balance_after 5221660.52 |
| VB-BANK-026 | On which day was the first transaction of the account PK00SYNT0000000000002003? | 2026-02-05 | correct: Day: 2026-02-05. \| first row: day 2026-02-05 |
| VB-BANK-027 | On which day was the last transaction of the account PK00SYNT0000000000002003? | 2026-04-17 | correct: Day: 2026-04-17. \| first row: day 2026-04-17 |
| VB-BANK-028 | Who did the account PK00SYNT0000000000002003 pay the most money to? | PK00SYNT0000000000005003 (before PK00SYNT0000000000005001, PK00SYNT0000000000005010) - 587,069.43 | correct: Counterparty PK00SYNT0000000000005003; Total amount 587,069.43. \| first row: counterparty PK00SYNT000000000000 |
| VB-BANK-029 | How many different accounts or numbers did the account PK00SYNT0000000000002003 pay? | 11 | correct: Count: 11. \| first row: count 11 |
| VB-BANK-030 | How many debits of the account PK00SYNT0000000000002003 were above 192250 rupees? | 5 | correct: N: 5. \| first row: n 5 |
| VB-BANK-031 | How many transactions does the account PK00SYNT0000000000002007 have? | 36 | correct: N: 36. \| first row: n 36 |
| VB-BANK-032 | What is the total amount credited to the account PK00SYNT0000000000002007? | 677,352.01 rupees | correct: Total credited: 677,352.01. \| first row: total_credited 677352.01 |
| VB-BANK-033 | What is the total amount debited from the account PK00SYNT0000000000002007? | 854,570.31 rupees | correct: Total debited: 854,570.31. \| first row: total_debited 854570.31 |
| VB-BANK-034 | What is the largest single debit of the account PK00SYNT0000000000002007? | 87,170.53 rupees | correct: Largest: 87,170.53. \| first row: largest 87170.53 |
| VB-BANK-035 | What was the balance after the last transaction of the account PK00SYNT0000000000002007? | 5,912,781.70 rupees | correct: Balance: 5,912,781.7. \| first row: balance 5912781.7 |
| VB-BANK-036 | On which day was the first transaction of the account PK00SYNT0000000000002007? | 2026-02-02 | correct: Day: 2026-02-02. \| first row: day 2026-02-02 |
| VB-BANK-037 | On which day was the last transaction of the account PK00SYNT0000000000002007? | 2026-04-17 | correct: Day: 2026-04-17. \| first row: day 2026-04-17 |
| VB-BANK-038 | Who did the account PK00SYNT0000000000002007 pay the most money to? | PK00SYNT0000000000005007 (before PK00SYNT0000000000005002, PK00SYNT0000000000005003) - 187,574.41 | correct: Counterparty PK00SYNT0000000000005007; Total amount 187,574.41. \| first row: counterparty PK00SYNT000000000000 |
| VB-BANK-039 | How many different accounts or numbers did the account PK00SYNT0000000000002007 pay? | 10 | correct: Count: 10. \| first row: count 10 |
| VB-BANK-040 | How many debits of the account PK00SYNT0000000000002007 were above 43585 rupees? | 10 | correct: N: 10. \| first row: n 10 |
| VB-BANK-041 | How many transactions does the account PK00SYNT0000000000002005 have? | 36 | correct: N: 36. \| first row: n 36 |
| VB-BANK-042 | What is the total amount credited to the account PK00SYNT0000000000002005? | 660,455.52 rupees | correct: Total credited: 660,455.52. \| first row: total_credited 660455.52 |
| VB-BANK-043 | What is the total amount debited from the account PK00SYNT0000000000002005? | 756,235.72 rupees | correct: Total debited: 756,235.72. \| first row: total_debited 756235.72 |
| VB-BANK-044 | What is the largest single debit of the account PK00SYNT0000000000002005? | 86,869.78 rupees | correct: Largest: 86,869.78. \| first row: largest 86869.78 |
| VB-BANK-045 | What was the balance after the last transaction of the account PK00SYNT0000000000002005? | 5,934,219.80 rupees | correct: Balance after: 5,934,219.8. \| first row: balance_after 5934219.8 |
| VB-BANK-046 | On which day was the first transaction of the account PK00SYNT0000000000002005? | 2026-02-05 | correct: First day: 2026-02-05. \| first row: first_day 2026-02-05 |
| VB-BANK-047 | On which day was the last transaction of the account PK00SYNT0000000000002005? | 2026-04-17 | correct: Day: 2026-04-17. \| first row: day 2026-04-17 |
| VB-BANK-048 | Who did the account PK00SYNT0000000000002005 pay the most money to? | PK00SYNT0000000000005002 (before PK00SYNT0000000000007002, PK00SYNT0000000000005008) - 244,370.39 | correct: Counterparty PK00SYNT0000000000005002; Total amount 244,370.39. \| first row: counterparty PK00SYNT000000000000 |
| VB-BANK-049 | How many different accounts or numbers did the account PK00SYNT0000000000002005 pay? | 13 | correct: Count: 13. \| first row: count 13 |
| VB-BANK-050 | Which accounts or numbers sent money to the account PK00SYNT0000000000002005? | PK00SYNT0000000000005001, PK00SYNT0000000000005002, PK00SYNT0000000000005004, PK00SYNT0000000000005007, PK00SYNT0000000000005008, PK00SYNT0000000000005009, PK00SYNT0000000000009002 | abstained: I did not give an answer, because I could not show that the query I wrote asks everything you asked: the quest |
| VB-BANK-051 | How many debits of the account PK00SYNT0000000000002005 were above 43434 rupees? | 5 | correct: N: 5. \| first row: n 5 |
| VB-BANK-052 | How many transactions does the account PK00SYNT0000000000002001 have? | 35 | correct: N: 35. \| first row: n 35 |
| VB-BANK-053 | What is the total amount credited to the account PK00SYNT0000000000002001? | 856,554.72 rupees | correct: Total credited: 856,554.72. \| first row: total_credited 856554.72 |
| VB-BANK-054 | What is the total amount debited from the account PK00SYNT0000000000002001? | 957,419.07 rupees | correct: Total debited: 957,419.07. \| first row: total_debited 957419.07 |
| VB-BANK-055 | What is the largest single debit of the account PK00SYNT0000000000002001? | 88,112.71 rupees | correct: Largest: 88,112.71. \| first row: largest 88112.71 |
| VB-BANK-056 | What was the balance after the last transaction of the account PK00SYNT0000000000002001? | 5,979,135.65 rupees | correct: Balance after: 5,979,135.65. \| first row: balance_after 5979135.65 |
| VB-BANK-057 | On which day was the first transaction of the account PK00SYNT0000000000002001? | 2026-02-04 | correct: First day: 2026-02-04. \| first row: first_day 2026-02-04 |
| VB-BANK-058 | On which day was the last transaction of the account PK00SYNT0000000000002001? | 2026-04-16 | correct: Day: 2026-04-16. \| first row: day 2026-04-16 |
| VB-BANK-059 | Who did the account PK00SYNT0000000000002001 pay the most money to? | PK00SYNT0000000000005003 (before PK00SYNT0000000000005009, PK00SYNT0000000000005007) - 171,945.64 | correct: Counterparty PK00SYNT0000000000005003; Total amount 171,945.64. \| first row: counterparty PK00SYNT000000000000 |
| VB-BANK-060 | How many different accounts or numbers did the account PK00SYNT0000000000002001 pay? | 11 | correct: Count: 11. \| first row: count 11 |
| VB-BANK-061 | How many debits of the account PK00SYNT0000000000002001 were above 44056 rupees? | 12 | correct: N: 12. \| first row: n 12 |
| VB-BANK-062 | How many transactions does the account PK00SYNT0000000000002009 have? | 35 | correct: N: 35. \| first row: n 35 |
| VB-BANK-063 | What is the total amount credited to the account PK00SYNT0000000000002009? | 516,585.01 rupees | correct: Total credited: 516,585.01. \| first row: total_credited 516585.01 |
| VB-BANK-064 | What is the total amount debited from the account PK00SYNT0000000000002009? | 863,186.33 rupees | correct: Total debited: 863,186.33. \| first row: total_debited 863186.33 |
| VB-BANK-065 | What is the largest single debit of the account PK00SYNT0000000000002009? | 85,988.25 rupees | correct: Largest: 85,988.25. \| first row: largest 85988.25 |
| VB-BANK-066 | What was the balance after the last transaction of the account PK00SYNT0000000000002009? | 5,693,398.68 rupees | correct: Balance after: 5,693,398.68. \| first row: balance_after 5693398.68 |
| VB-BANK-067 | On which day was the first transaction of the account PK00SYNT0000000000002009? | 2026-02-05 | correct: First day: 2026-02-05. \| first row: first_day 2026-02-05 |
| VB-BANK-068 | On which day was the last transaction of the account PK00SYNT0000000000002009? | 2026-04-16 | correct: Day: 2026-04-16. \| first row: day 2026-04-16 |
| VB-BANK-069 | Who did the account PK00SYNT0000000000002009 pay the most money to? | PK00SYNT0000000000005008 (before PK00SYNT0000000000005009, PK00SYNT0000000000005006) - 160,317.61 | correct: Counterparty PK00SYNT0000000000005008; Total amount 160,317.61. \| first row: counterparty PK00SYNT000000000000 |
| VB-BANK-070 | How many different accounts or numbers did the account PK00SYNT0000000000002009 pay? | 11 | correct: Count: 11. \| first row: count 11 |
| VB-BANK-071 | How many debits of the account PK00SYNT0000000000002009 were above 42994 rupees? | 10 | correct: N: 10. \| first row: n 10 |
| VB-BANK-072 | How many transactions does the account PK00SYNT0000000000002011 have? | 34 | correct: N: 34. \| first row: n 34 |
| VB-BANK-073 | What is the total amount credited to the account PK00SYNT0000000000002011? | 630,905.10 rupees | correct: Total credited: 630,905.1. \| first row: total_credited 630905.1 |
| VB-BANK-074 | What is the total amount debited from the account PK00SYNT0000000000002011? | 636,579.84 rupees | correct: Total debited: 636,579.84. \| first row: total_debited 636579.84 |
| VB-BANK-075 | What is the largest single debit of the account PK00SYNT0000000000002011? | 87,403.32 rupees | correct: Largest: 87,403.32. \| first row: largest 87403.32 |
| VB-BANK-076 | What was the balance after the last transaction of the account PK00SYNT0000000000002011? | 6,104,325.26 rupees | correct: Balance after: 6,104,325.26. \| first row: balance_after 6104325.26 |
| VB-BANK-077 | On which day was the first transaction of the account PK00SYNT0000000000002011? | 2026-02-05 | correct: First day: 2026-02-05. \| first row: first_day 2026-02-05 |
| VB-BANK-078 | On which day was the last transaction of the account PK00SYNT0000000000002011? | 2026-04-17 | correct: Day: 2026-04-17. \| first row: day 2026-04-17 |
| VB-BANK-079 | Who did the account PK00SYNT0000000000002011 pay the most money to? | PK00SYNT0000000000005003 (before PK00SYNT0000000000005006, PK00SYNT0000000000005008) - 142,680.70 | correct: Counterparty PK00SYNT0000000000005003; Transactions 6; Total amount 142,680.7. \| first row: counterparty PK00S |
| VB-BANK-080 | How many different accounts or numbers did the account PK00SYNT0000000000002011 pay? | 10 | correct: Count: 10. \| first row: count 10 |
| VB-BANK-081 | How many debits of the account PK00SYNT0000000000002011 were above 43701 rupees? | 7 | correct: N: 7. \| first row: n 7 |
| VB-BANK-082 | How many transactions does the account PK00SYNT0000000000002008 have? | 32 | correct: N: 32. \| first row: n 32 |
| VB-BANK-083 | What is the total amount credited to the account PK00SYNT0000000000002008? | 699,422.95 rupees | correct: Total credited: 699,422.95. \| first row: total_credited 699422.95 |
| VB-BANK-084 | What is the total amount debited from the account PK00SYNT0000000000002008? | 820,306.92 rupees | correct: Total debited: 820,306.92. \| first row: total_debited 820306.92 |
| VB-BANK-085 | What is the largest single debit of the account PK00SYNT0000000000002008? | 87,250.09 rupees | correct: Largest: 87,250.09. \| first row: largest 87250.09 |
| VB-BANK-086 | What was the balance after the last transaction of the account PK00SYNT0000000000002008? | 5,939,116.03 rupees | correct: Balance: 5,939,116.03. \| first row: balance 5939116.03 |
| VB-BANK-087 | On which day was the first transaction of the account PK00SYNT0000000000002008? | 2026-02-02 | correct: First transaction day: 2026-02-02. \| first row: first_transaction_day 2026-02-02 |
| VB-BANK-088 | On which day was the last transaction of the account PK00SYNT0000000000002008? | 2026-04-15 | correct: Day: 2026-04-15. \| first row: day 2026-04-15 |
| VB-BANK-089 | Who did the account PK00SYNT0000000000002008 pay the most money to? | PK00SYNT0000000000005002 (before PK00SYNT0000000000005007, PK00SYNT0000000000005000) - 152,336.17 | correct: Counterparty PK00SYNT0000000000005002; Transactions 3; Total amount 152,336.17. \| first row: counterparty PK00 |
| VB-BANK-090 | How many different accounts or numbers did the account PK00SYNT0000000000002008 pay? | 11 | correct: Count: 11. \| first row: count 11 |
| VB-BANK-091 | Which accounts or numbers sent money to the account PK00SYNT0000000000002008? | PK00SYNT0000000000005001, PK00SYNT0000000000005005, PK00SYNT0000000000005006, PK00SYNT0000000000005008, PK00SYNT0000000000005009, PK00SYNT0000000000005011, PK00SYNT0000000000009002 | abstained: I did not give an answer, because I could not show that the query I wrote asks everything you asked: the quest |
| VB-BANK-092 | How many debits of the account PK00SYNT0000000000002008 were above 43625 rupees? | 10 | correct: N: 10. \| first row: n 10 |
| VB-BANK-093 | How many transactions does the account PK00SYNT0000000000002006 have? | 30 | correct: N: 30. \| first row: n 30 |
| VB-BANK-094 | What is the total amount credited to the account PK00SYNT0000000000002006? | 797,897.72 rupees | correct: Total credited: 797,897.72. \| first row: total_credited 797897.72 |
| VB-BANK-095 | What is the total amount debited from the account PK00SYNT0000000000002006? | 483,705.65 rupees | correct: Total debited: 483,705.65. \| first row: total_debited 483705.65 |
| VB-BANK-096 | What is the largest single debit of the account PK00SYNT0000000000002006? | 71,619.85 rupees | correct: Largest: 71,619.85. \| first row: largest 71619.85 |
| VB-BANK-097 | What was the balance after the last transaction of the account PK00SYNT0000000000002006? | 6,414,192.07 rupees | correct: Balance: 6,414,192.07. \| first row: balance 6414192.07 |
| VB-BANK-098 | On which day was the first transaction of the account PK00SYNT0000000000002006? | 2026-02-02 | correct: First transaction day: 2026-02-02. \| first row: first_transaction_day 2026-02-02 |
| VB-BANK-099 | On which day was the last transaction of the account PK00SYNT0000000000002006? | 2026-04-17 | correct: Day: 2026-04-17. \| first row: day 2026-04-17 |
| VB-BANK-100 | Who did the account PK00SYNT0000000000002006 pay the most money to? | PK00SYNT0000000000005004 (before PK00SYNT0000000000005007, PK00SYNT0000000000005006) - 107,949.34 | correct: Counterparty PK00SYNT0000000000005004; Total amount 107,949.34. \| first row: counterparty PK00SYNT000000000000 |
| VB-BANK-101 | How many different accounts or numbers did the account PK00SYNT0000000000002006 pay? | 9 | correct: Count: 9. \| first row: count 9 |
| VB-BANK-102 | How many debits of the account PK00SYNT0000000000002006 were above 35809 rupees? | 8 | correct: N: 8. \| first row: n 8 |
| VB-BANK-103 | How many transactions does the account PK00SYNT0000000000002002 have? | 28 | correct: N: 28. \| first row: n 28 |
| VB-BANK-104 | What is the total amount credited to the account PK00SYNT0000000000002002? | 483,381.14 rupees | correct: Total credited: 483,381.14. \| first row: total_credited 483381.14 |
| VB-BANK-105 | What is the total amount debited from the account PK00SYNT0000000000002002? | 893,711.55 rupees | correct: Total debited: 893,711.55. \| first row: total_debited 893711.55 |
| VB-BANK-106 | What is the largest single debit of the account PK00SYNT0000000000002002? | 83,926.58 rupees | correct: Largest: 83,926.58. \| first row: largest 83926.58 |
| VB-BANK-107 | What was the balance after the last transaction of the account PK00SYNT0000000000002002? | 5,659,669.59 rupees | correct: Balance after: 5,659,669.59. \| first row: balance_after 5659669.59 |
| VB-BANK-108 | On which day was the first transaction of the account PK00SYNT0000000000002002? | 2026-02-02 | correct: Day: 2026-02-02. \| first row: day 2026-02-02 |
| VB-BANK-109 | On which day was the last transaction of the account PK00SYNT0000000000002002? | 2026-04-17 | correct: Day: 2026-04-17. \| first row: day 2026-04-17 |
| VB-BANK-110 | Who did the account PK00SYNT0000000000002002 pay the most money to? | PK00SYNT0000000000005001 (before PK00SYNT0000000000005006, PK00SYNT0000000000005004) - 146,549.29 | correct: Counterparty PK00SYNT0000000000005001; Total amount 146,549.29. \| first row: counterparty PK00SYNT000000000000 |
| VB-BANK-111 | How many different accounts or numbers did the account PK00SYNT0000000000002002 pay? | 11 | correct: Count: 11. \| first row: count 11 |
| VB-BANK-112 | Which accounts or numbers sent money to the account PK00SYNT0000000000002002? | PK00SYNT0000000000005005, PK00SYNT0000000000005007, PK00SYNT0000000000005010, PK00SYNT0000000000005011, PK00SYNT0000000000009002 | abstained: I did not give an answer, because I could not show that the query I wrote asks everything you asked: the quest |
| VB-BANK-113 | How many debits of the account PK00SYNT0000000000002002 were above 41963 rupees? | 12 | correct: N: 12. \| first row: n 12 |
| VB-BANK-114 | How many transactions does the account PK00SYNT0000000000002000 have? | 28 | correct: N: 28. \| first row: n 28 |
| VB-BANK-115 | What is the total amount credited to the account PK00SYNT0000000000002000? | 683,990.80 rupees | correct: Total credited: 683,990.8. \| first row: total_credited 683990.8 |
| VB-BANK-116 | What is the total amount debited from the account PK00SYNT0000000000002000? | 595,024.02 rupees | correct: Total debited: 595,024.02. \| first row: total_debited 595024.02 |
| VB-BANK-117 | What is the largest single debit of the account PK00SYNT0000000000002000? | 89,254.51 rupees | correct: Largest: 89,254.51. \| first row: largest 89254.51 |
| VB-BANK-118 | What was the balance after the last transaction of the account PK00SYNT0000000000002000? | 6,138,966.78 rupees | correct: Balance: 6,138,966.78. \| first row: balance 6138966.78 |
| VB-BANK-119 | On which day was the first transaction of the account PK00SYNT0000000000002000? | 2026-02-02 | correct: First transaction day: 2026-02-02. \| first row: first_transaction_day 2026-02-02 |
| VB-BANK-120 | On which day was the last transaction of the account PK00SYNT0000000000002000? | 2026-04-09 | correct: Day: 2026-04-09. \| first row: day 2026-04-09 |
| VB-BANK-121 | Who did the account PK00SYNT0000000000002000 pay the most money to? | PK00SYNT0000000000005011 (before PK00SYNT0000000000005008, PK00SYNT0000000000005004) - 142,095.46 | correct: Counterparty PK00SYNT0000000000005011; Total amount 142,095.46. \| first row: counterparty PK00SYNT000000000000 |
| VB-BANK-122 | How many different accounts or numbers did the account PK00SYNT0000000000002000 pay? | 9 | correct: Count: 9. \| first row: count 9 |
| VB-BANK-123 | Which accounts or numbers sent money to the account PK00SYNT0000000000002000? | PK00SYNT0000000000005000, PK00SYNT0000000000005001, PK00SYNT0000000000005003, PK00SYNT0000000000005006, PK00SYNT0000000000005009, PK00SYNT0000000000005011, PK00SYNT0000000000009002 | abstained: I did not give an answer, because I could not show that the query I wrote asks everything you asked: the quest |
| VB-BANK-124 | How many debits of the account PK00SYNT0000000000002000 were above 44627 rupees? | 6 | correct: N: 6. \| first row: n 6 |

## Vehicle plates and cameras

How often a plate was seen, at which cameras, when first and last; how many sightings a camera recorded and how many different plates; the plate seen the most.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-PLAT-001 | How many times was the plate SYNT2905 seen? | 50 | correct: N: 50. \| first row: n 50 |
| VB-PLAT-002 | At which cameras was the plate SYNT2905 seen? | CAM-02, CAM-03, CAM-04, CAM-05, CAM-08 | correct: 5 rows. \| first row: camera_id CAM-02; camera_label Camera 2 |
| VB-PLAT-003 | When was the plate SYNT2905 first seen? | 2026-02-11 17:24:19 | correct: First seen: 2026-02-11 17:24:19 PKT. \| first row: first_seen 2026-02-11 17:24:19 PKT |
| VB-PLAT-004 | When was the plate SYNT2905 last seen? | 2026-04-15 07:35:42 | correct: Last seen: 2026-04-15 07:35:42. \| first row: last_seen 2026-04-15 07:35:42 |
| VB-PLAT-005 | How many times was the plate SYNT4024 seen? | 50 | correct: N: 50. \| first row: n 50 |
| VB-PLAT-006 | At which cameras was the plate SYNT4024 seen? | CAM-01, CAM-03, CAM-05, CAM-07, CAM-08 | correct: 5 rows. \| first row: camera_id CAM-03; camera_label Camera 3 |
| VB-PLAT-007 | When was the plate SYNT4024 first seen? | 2026-02-16 23:44:49 | correct: First seen: 2026-02-16 23:44:49 PKT. \| first row: first_seen 2026-02-16 23:44:49 PKT |
| VB-PLAT-008 | When was the plate SYNT4024 last seen? | 2026-04-09 00:44:37 | correct: Last seen: 2026-04-09 00:44:37. \| first row: last_seen 2026-04-09 00:44:37 |
| VB-PLAT-009 | How many times was the plate SYNT2019 seen? | 30 | correct: N: 30. \| first row: n 30 |
| VB-PLAT-010 | At which cameras was the plate SYNT2019 seen? | CAM-02, CAM-03, CAM-04, CAM-05, CAM-08 | correct: 5 rows. \| first row: camera_id CAM-02; camera_label Camera 2 |
| VB-PLAT-011 | When was the plate SYNT2019 first seen? | 2026-02-26 14:55:15 | correct: First seen: 2026-02-26 14:55:15 PKT. \| first row: first_seen 2026-02-26 14:55:15 PKT |
| VB-PLAT-012 | When was the plate SYNT2019 last seen? | 2026-03-31 19:27:56 | correct: Last seen: 2026-03-31 19:27:56. \| first row: last_seen 2026-03-31 19:27:56 |
| VB-PLAT-013 | How many times was the plate SYNT2233 seen? | 24 | correct: N: 24. \| first row: n 24 |
| VB-PLAT-014 | At which cameras was the plate SYNT2233 seen? | CAM-01, CAM-05, CAM-06, CAM-08 | correct: 4 rows. \| first row: camera_id CAM-01; camera_label Camera 1 |
| VB-PLAT-015 | When was the plate SYNT2233 first seen? | 2026-02-03 17:21:55 | correct: First seen: 2026-02-03 17:21:55 PKT. \| first row: first_seen 2026-02-03 17:21:55 PKT |
| VB-PLAT-016 | When was the plate SYNT2233 last seen? | 2026-04-14 12:29:14 | correct: Last seen: 2026-04-14 12:29:14. \| first row: last_seen 2026-04-14 12:29:14 |
| VB-PLAT-017 | How many times was the plate SYNT9150 seen? | 14 | correct: N: 14. \| first row: n 14 |
| VB-PLAT-018 | At which cameras was the plate SYNT9150 seen? | CAM-01, CAM-02 | correct: 2 rows. \| first row: camera_id CAM-01; camera_label Camera 1 |
| VB-PLAT-019 | When was the plate SYNT9150 first seen? | 2026-02-05 21:14:18 | correct: First seen: 2026-02-05 21:14:18 PKT. \| first row: first_seen 2026-02-05 21:14:18 PKT |
| VB-PLAT-020 | When was the plate SYNT9150 last seen? | 2026-04-12 14:08:44 | correct: Last seen: 2026-04-12 14:08:44 PKT. \| first row: last_seen 2026-04-12 14:08:44 PKT |
| VB-PLAT-021 | How many times was the plate SYNT6147 seen? | 12 | correct: N: 12. \| first row: n 12 |
| VB-PLAT-022 | At which cameras was the plate SYNT6147 seen? | CAM-05, CAM-07 | correct: 2 rows. \| first row: camera_id CAM-05; camera_label Camera 5 |
| VB-PLAT-023 | When was the plate SYNT6147 first seen? | 2026-03-14 20:47:27 | correct: First seen: 2026-03-14 20:47:27 PKT. \| first row: first_seen 2026-03-14 20:47:27 PKT |
| VB-PLAT-024 | When was the plate SYNT6147 last seen? | 2026-04-15 17:27:08 | correct: Last seen: 2026-04-15 17:27:08. \| first row: last_seen 2026-04-15 17:27:08 |
| VB-PLAT-025 | How many times was the plate SYNT4O24 seen? | 4 | correct: N: 4. \| first row: n 4 |
| VB-PLAT-026 | At which cameras was the plate SYNT4O24 seen? | CAM-01, CAM-08, CAM-09 | correct: 3 rows. \| first row: camera_id CAM-01; camera_label Camera 1 |
| VB-PLAT-027 | When was the plate SYNT4O24 first seen? | 2026-02-04 15:53:32 | correct: First seen: 2026-02-04 15:53:32 PKT. \| first row: first_seen 2026-02-04 15:53:32 PKT |
| VB-PLAT-028 | When was the plate SYNT4O24 last seen? | 2026-04-08 07:39:31 | correct: 1 sighting of the plate SYNT4O24 at 1 camera. \| first row: camera_id CAM-08; camera_label Camera 8; lat 31.543 |
| VB-PLAT-029 | How many times was the plate SYNT4016 seen? | 4 | correct: N: 4. \| first row: n 4 |
| VB-PLAT-030 | At which cameras was the plate SYNT4016 seen? | CAM-03, CAM-04, CAM-08 | correct: 3 rows. \| first row: camera_id CAM-03; camera_label Camera 3 |
| VB-PLAT-031 | When was the plate SYNT4016 first seen? | 2026-03-02 00:58:25 | correct: First seen: 2026-03-02 00:58:25 PKT. \| first row: first_seen 2026-03-02 00:58:25 PKT |
| VB-PLAT-032 | When was the plate SYNT4016 last seen? | 2026-03-17 23:44:36 | correct: Last seen: 2026-03-17 23:44:36. \| first row: last_seen 2026-03-17 23:44:36 |
| VB-PLAT-033 | How many plate sightings did the camera CAM-01 record? | 36 | correct: Value: 36. \| first row: value 36 |
| VB-PLAT-034 | How many different plates did the camera CAM-01 see? | 16 | correct: Value: 16. \| first row: value 16 |
| VB-PLAT-035 | How many plate sightings did the camera CAM-02 record? | 31 | correct: Value: 31. \| first row: value 31 |
| VB-PLAT-036 | How many different plates did the camera CAM-02 see? | 9 | correct: Value: 9. \| first row: value 9 |
| VB-PLAT-037 | How many plate sightings did the camera CAM-03 record? | 33 | correct: Value: 33. \| first row: value 33 |
| VB-PLAT-038 | How many different plates did the camera CAM-03 see? | 9 | correct: Value: 9. \| first row: value 9 |
| VB-PLAT-039 | How many plate sightings did the camera CAM-04 record? | 33 | correct: Value: 33. \| first row: value 33 |
| VB-PLAT-040 | How many different plates did the camera CAM-04 see? | 16 | correct: Value: 16. \| first row: value 16 |
| VB-PLAT-041 | How many plate sightings did the camera CAM-05 record? | 49 | correct: Value: 49. \| first row: value 49 |
| VB-PLAT-042 | How many different plates did the camera CAM-05 see? | 16 | correct: Value: 16. \| first row: value 16 |
| VB-PLAT-043 | How many plate sightings did the camera CAM-06 record? | 16 | correct: Value: 16. \| first row: value 16 |
| VB-PLAT-044 | How many different plates did the camera CAM-06 see? | 9 | correct: Value: 9. \| first row: value 9 |
| VB-PLAT-045 | How many plate sightings did the camera CAM-07 record? | 33 | correct: Value: 33. \| first row: value 33 |
| VB-PLAT-046 | How many different plates did the camera CAM-07 see? | 16 | correct: Value: 16. \| first row: value 16 |
| VB-PLAT-047 | How many plate sightings did the camera CAM-08 record? | 45 | correct: Value: 45. \| first row: value 45 |
| VB-PLAT-048 | How many different plates did the camera CAM-08 see? | 15 | correct: Value: 15. \| first row: value 15 |
| VB-PLAT-049 | How many plate sightings did the camera CAM-09 record? | 10 | correct: Value: 10. \| first row: value 10 |
| VB-PLAT-050 | How many different plates did the camera CAM-09 see? | 10 | correct: Value: 10. \| first row: value 10 |
| VB-PLAT-051 | Which plate was seen the most? | SYNT2905 (before SYNT4024) - 50 | wrong: 1 result: SYNT4024 (50). \| first row: label SYNT4024; value 50 |

## Handsets

Which handset (IMEI) a number was seen with and with how many.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-HAND-001 | Which handset (IMEI) was 923000001005 seen with? | 000000000010017 | correct: Imei: 000000000010017. \| first row: imei 000000000010017 |
| VB-HAND-002 | How many different handsets was 923000001005 seen with? | 1 | correct: Different handsets: 1. \| first row: different_handsets 1 |
| VB-HAND-003 | Which handset (IMEI) was 923000001002 seen with? | 000000000010025, 000000000200014 | correct: 2 rows. \| first row: imei 000000000200014 |
| VB-HAND-004 | How many different handsets was 923000001002 seen with? | 2 | correct: Different handsets: 2. \| first row: different_handsets 2 |
| VB-HAND-005 | Which handset (IMEI) was 923000001006 seen with? | 000000000010066, 000000000200048 | correct: 2 rows. \| first row: imei 000000000200048 |
| VB-HAND-006 | How many different handsets was 923000001006 seen with? | 2 | correct: Different handsets: 2. \| first row: different_handsets 2 |
| VB-HAND-007 | Which handset (IMEI) was 923000001010 seen with? | 000000000010108 | correct: 2 rows. \| first row: imei 000000000010108 |
| VB-HAND-008 | How many different handsets was 923000001010 seen with? | 1 | correct: Count handsets: 1. \| first row: count_handsets 1 |
| VB-HAND-009 | Which handset (IMEI) was 923000001000 seen with? | 000000000010009 | correct: Imei: 000000000010009. \| first row: imei 000000000010009 |
| VB-HAND-010 | How many different handsets was 923000001000 seen with? | 1 | correct: Different handsets: 1. \| first row: different_handsets 1 |
| VB-HAND-011 | Which handset (IMEI) was 923000001007 seen with? | 00000000001007 | correct: Imei: 00000000001007. \| first row: imei 00000000001007 |
| VB-HAND-012 | How many different handsets was 923000001007 seen with? | 1 | correct: Different handsets: 1. \| first row: different_handsets 1 |
| VB-HAND-013 | Which handset (IMEI) was 923000001009 seen with? | 000000000010090 | correct: Imei: 000000000010090. \| first row: imei 000000000010090 |
| VB-HAND-014 | How many different handsets was 923000001009 seen with? | 1 | correct: Different handsets: 1. \| first row: different_handsets 1 |
| VB-HAND-015 | Which handset (IMEI) was 923000001001 seen with? | 000000000010017 | correct: Imei: 000000000010017. \| first row: imei 000000000010017 |
| VB-HAND-016 | How many different handsets was 923000001001 seen with? | 1 | correct: Different handsets: 1. \| first row: different_handsets 1 |
| VB-HAND-017 | Which handset (IMEI) was 923000001008 seen with? | 000000000010041 | correct: Imei: 000000000010041. \| first row: imei 000000000010041 |
| VB-HAND-018 | How many different handsets was 923000001008 seen with? | 1 | correct: Different handsets: 1. \| first row: different_handsets 1 |
| VB-HAND-019 | Which handset (IMEI) was 923000001003 seen with? | 00000000001003 | correct: Imei: 00000000001003. \| first row: imei 00000000001003 |
| VB-HAND-020 | How many different handsets was 923000001003 seen with? | 1 | correct: Different handsets: 1. \| first row: different_handsets 1 |
| VB-HAND-021 | Which handset (IMEI) was 923000001011 seen with? | 00000000001011 | correct: Imei: 00000000001011. \| first row: imei 00000000001011 |
| VB-HAND-022 | How many different handsets was 923000001011 seen with? | 1 | correct: Count: 1. \| first row: count 1 |
| VB-HAND-023 | Which handset (IMEI) was 923000001004 seen with? | 000000000010041 | correct: Imei: 000000000010041. \| first row: imei 000000000010041 |
| VB-HAND-024 | How many different handsets was 923000001004 seen with? | 1 | correct: Different handsets: 1. \| first row: different_handsets 1 |

## Cell sites

The cell site a number used most and how many different cell sites it used. A cell site is a coverage area, not a position.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-CELL-001 | Which cell site did 923000001005 use the most? | ?:10221:10629 (before ?:10273:10777, ?:10234:10666) - 338 | correct: 1 cell site. \| first row: cell_key ?:10221:10629; lat 31.48070000000001; lon 74.30454999999998; radius_m None; |
| VB-CELL-002 | How many different cell sites did 923000001005 use? | 6 | correct: Different cell sites: 6. \| first row: different_cell_sites 6 |
| VB-CELL-003 | Which cell site did 923000001002 use the most? | ?:?:3003665515 (before ?:?:3003141870, ?:?:3003246599) - 362 | correct: 1 cell site. \| first row: cell_key ?:?:3003665515; lat 31.501400000000043; lon 74.35253999999995; radius_m Non |
| VB-CELL-004 | How many different cell sites did 923000001002 use? | 5 | correct: Different cell sites: 5. \| first row: different_cell_sites 5 |
| VB-CELL-005 | Which cell site did 923000001006 use the most? | ?:?:3003456057 (before ?:?:3002513496, ?:?:3003665515) - 307 | correct: 1 cell site. \| first row: cell_key ?:?:3003456057; lat 31.501399999999997; lon 74.30564699999998; radius_m Non |
| VB-CELL-006 | How many different cell sites did 923000001006 use? | 5 | correct: Different cell sites: 5. \| first row: different_cell_sites 5 |
| VB-CELL-007 | Which cell site did 923000001010 use the most? | ?:?:3003665515 (before ?:?:3002513496, ?:?:3002618225) - 274 | correct: 1 cell site. \| first row: cell_key ?:?:3003665515; lat 31.501400000000043; lon 74.35253999999995; radius_m Non |
| VB-CELL-008 | How many different cell sites did 923000001010 use? | 5 | correct: Different cell sites: 5. \| first row: different_cell_sites 5 |
| VB-CELL-009 | Which cell site did 923000001000 use the most? | ?:?:103d13e (before ?:?:108i18j, ?:?:104e14f) - 203 | correct: 1 cell site. \| first row: cell_key ?:?:103d13e; lat 31.460000000000022; lon 74.35033999999996; radius_m None;  |
| VB-CELL-010 | How many different cell sites did 923000001000 use? | 4 | correct: Different cell sites: 4. \| first row: different_cell_sites 4 |
| VB-CELL-011 | Which cell site did 923000001007 use the most? | ?:518:10300922 (before ?:606:10364274, ?:617:10372193) - 195 | correct: 1 cell site. \| first row: cell_key ?:518:10300922; lat 31.462099999999992; lon 74.33019052118641; radius_m Non |
| VB-CELL-012 | How many different cell sites did 923000001007 use? | 4 | correct: Different cell sites: 4. \| first row: different_cell_sites 4 |
| VB-CELL-013 | Which cell site did 923000001009 use the most? | ?:10156:10444 (before ?:10286:10814, ?:10169:10481) - 164 | correct: 1 cell site. \| first row: cell_key ?:10156:10444; lat 31.46069999999999; lon 74.2811; radius_m None; site_name |
| VB-CELL-014 | How many different cell sites did 923000001009 use? | 6 | correct: Different cell sites: 6. \| first row: different_cell_sites 6 |
| VB-CELL-015 | Which cell site did 923000001001 use the most? | ?:10286:10814 (before ?:10221:10629, ?:10299:10851) - 146 | correct: 1 cell site. \| first row: cell_key ?:10286:10814; lat 31.50070000000002; lon 74.32798999999997; radius_m None; |
| VB-CELL-016 | How many different cell sites did 923000001001 use? | 5 | correct: Different cell sites: 5. \| first row: different_cell_sites 5 |

## Registrations

Which SIM identity a number is registered with and the status of the registration. A registration is what a source file states, not who holds the number.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-REGI-001 | Which SIM identity (IMSI) is registered for the number 923000001004? | 001010000001004 | correct: Imsi: 001010000001004. \| first row: imsi 001010000001004 |
| VB-REGI-002 | What is the status of the registration of the number 923000001004? | active | correct: Status: active. \| first row: status active |
| VB-REGI-003 | Which SIM identity (IMSI) is registered for the number 923000001007? | 001010000001007 | correct: Imsi: 001010000001007. \| first row: imsi 001010000001007 |
| VB-REGI-004 | What is the status of the registration of the number 923000001007? | deactivated | correct: Status: deactivated. \| first row: status deactivated |
| VB-REGI-005 | Which SIM identity (IMSI) is registered for the number 923000001009? | 001010000001009 | correct: Imsi: 001010000001009. \| first row: imsi 001010000001009 |
| VB-REGI-006 | What is the status of the registration of the number 923000001009? | deactivated | correct: Status: deactivated. \| first row: status deactivated |
| VB-REGI-007 | Which SIM identity (IMSI) is registered for the number 923000001010? | 001010000001010 | correct: Imsi: 001010000001010. \| first row: imsi 001010000001010 |
| VB-REGI-008 | What is the status of the registration of the number 923000001010? | deactivated | correct: Status: deactivated. \| first row: status deactivated |
| VB-REGI-009 | Which SIM identity (IMSI) is registered for the number 923000001012? | 001010000001012 | correct: Imsi 001010000001012; Iccid 8900000000000010127; Status active; Valid from 2024-09-01 00:00:00 PKT; Valid to n |
| VB-REGI-010 | What is the status of the registration of the number 923000001012? | active | correct: Status: active. \| first row: status active |
| VB-REGI-011 | Which SIM identity (IMSI) is registered for the number 923000001013? | 001010000001013 | correct: Imsi: 001010000001013. \| first row: imsi 001010000001013 |
| VB-REGI-012 | What is the status of the registration of the number 923000001013? | active | correct: Status: active. \| first row: status active |
| VB-REGI-013 | Which SIM identity (IMSI) is registered for the number 923000001014? | 001010000001014 | correct: Imsi: 001010000001014. \| first row: imsi 001010000001014 |
| VB-REGI-014 | What is the status of the registration of the number 923000001014? | active | correct: Status: active. \| first row: status active |
| VB-REGI-015 | Which SIM identity (IMSI) is registered for the number 923000001015? | 001010000001015 | correct: Imsi: 001010000001015. \| first row: imsi 001010000001015 |
| VB-REGI-016 | What is the status of the registration of the number 923000001015? | unknown | correct: Status: unknown. \| first row: status unknown |

## The evidence files

How many files the case holds, in total and by type.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-INVE-001 | How many document files are in this case? | 4 | correct: N: 4. \| first row: n 4 |
| VB-INVE-002 | How many structured record files are in this case? | 42 | correct: N: 42. \| first row: n 42 |
| VB-INVE-003 | How many tabular files are in this case? | 3 | wrong: N: 4. \| first row: n 4 |
| VB-INVE-004 | How many text files are in this case? | 4 | wrong: N: 14. \| first row: n 14 |
| VB-INVE-005 | How many evidence files are in this case? | 53 | correct: N: 53. \| first row: n 53 |

