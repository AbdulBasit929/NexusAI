# Verified question bank: the Orion Lahore reference case

484 questions, each with the answer computed by plain SQL on the stored rows (never through the engine) and the live engine's answer scored against it. Synthetic data only.

| Family | Questions | Correct | Wrong | Declined or abstained | Not run |
|---|---|---|---|---|---|
| Calls of one subscriber | 149 | 1 | 0 | 0 | 148 |
| Text messages of one subscriber | 30 | 1 | 0 | 0 | 29 |
| A subscriber and one other number | 32 | 0 | 0 | 0 | 32 |
| Internet sessions | 31 | 0 | 0 | 0 | 31 |
| App usage | 6 | 0 | 0 | 0 | 6 |
| Bank and wallet accounts | 124 | 0 | 0 | 0 | 124 |
| Vehicle plates and cameras | 51 | 0 | 0 | 0 | 51 |
| Handsets | 24 | 0 | 0 | 0 | 24 |
| Cell sites | 16 | 0 | 0 | 0 | 16 |
| Registrations | 16 | 0 | 0 | 0 | 16 |
| The evidence files | 5 | 0 | 0 | 0 | 5 |
| **All** | **484** | **2** | **0** | **0** | **482** |

## Calls of one subscriber

How many voice calls, how long, when, to whom and on which day: counts of calls made and received, talk time, the longest and the average call, calls at night, calls over ten minutes, the first and last event, the number called most, the busiest day, how many different numbers were contacted.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-CALL-001 | How many voice calls does 923000001005 have in this case? | 694 | correct: N: 694. \| first row: n 694 |
| VB-CALL-002 | How many outgoing voice calls did 923000001005 make? | 520 | not run |
| VB-CALL-003 | How many incoming voice calls did 923000001005 receive? | 174 | not run |
| VB-CALL-004 | What is the total talk time of 923000001005 across all its voice calls, in seconds? | 69,639 seconds | not run |
| VB-CALL-005 | What is the duration of the longest voice call of 923000001005, in seconds? | 1,272 seconds | not run |
| VB-CALL-006 | What is the average duration of the voice calls of 923000001005, in seconds? | 100.34 seconds | not run |
| VB-CALL-007 | How many voice calls of 923000001005 lasted more than 10 minutes? | 4 | not run |
| VB-CALL-008 | How many voice calls of 923000001005 started between 00:00 and 04:59 Karachi time? | 55 | not run |
| VB-CALL-009 | When was the first call or message of 923000001005? | 2026-02-02 00:29:05 | not run |
| VB-CALL-010 | When was the last call or message of 923000001005? | 2026-04-17 22:59:16 | not run |
| VB-CALL-011 | Which number did 923000001005 call the most with outgoing voice calls? | 923000001036 (before 923000001093, 923000001074, 923000001050) - 105 | not run |
| VB-CALL-012 | Which day had the most voice calls for 923000001005? | 2026-02-17 (before 2026-04-01, 2026-04-02) - 25 | not run |
| VB-CALL-013 | How many different numbers did 923000001005 contact, leaving out services? | 11 | not run |
| VB-CALL-014 | How many voice calls does 923000001002 have in this case? | 473 | not run |
| VB-CALL-015 | How many outgoing voice calls did 923000001002 make? | 219 | not run |
| VB-CALL-016 | How many incoming voice calls did 923000001002 receive? | 253 | not run |
| VB-CALL-017 | What is the total talk time of 923000001002 across all its voice calls, in seconds? | 57,050 seconds | not run |
| VB-CALL-018 | What is the duration of the longest voice call of 923000001002, in seconds? | 784 seconds | not run |
| VB-CALL-019 | What is the average duration of the voice calls of 923000001002, in seconds? | 120.61 seconds | not run |
| VB-CALL-020 | How many voice calls of 923000001002 lasted more than 10 minutes? | 3 | not run |
| VB-CALL-021 | How many voice calls of 923000001002 started between 00:00 and 04:59 Karachi time? | 17 | not run |
| VB-CALL-022 | When was the first call or message of 923000001002? | 2026-02-02 06:05:32 | not run |
| VB-CALL-023 | When was the last call or message of 923000001002? | 2026-04-17 21:45:15 | not run |
| VB-CALL-024 | Which number did 923000001002 call the most with outgoing voice calls? | 923000001012 (before 923000001054, 923000001048, 923000001032) - 47 | not run |
| VB-CALL-025 | How many different numbers did 923000001002 contact, leaving out services? | 20 | not run |
| VB-CALL-026 | How many voice calls does 923000001006 have in this case? | 437 | not run |
| VB-CALL-027 | How many outgoing voice calls did 923000001006 make? | 238 | not run |
| VB-CALL-028 | How many incoming voice calls did 923000001006 receive? | 197 | not run |
| VB-CALL-029 | What is the total talk time of 923000001006 across all its voice calls, in seconds? | 47,201 seconds | not run |
| VB-CALL-030 | What is the duration of the longest voice call of 923000001006, in seconds? | 548 seconds | not run |
| VB-CALL-031 | What is the average duration of the voice calls of 923000001006, in seconds? | 108.01 seconds | not run |
| VB-CALL-032 | How many voice calls of 923000001006 lasted more than 10 minutes? | 0 | not run |
| VB-CALL-033 | How many voice calls of 923000001006 started between 00:00 and 04:59 Karachi time? | 46 | not run |
| VB-CALL-034 | When was the first call or message of 923000001006? | 2026-02-02 07:28:45 | not run |
| VB-CALL-035 | When was the last call or message of 923000001006? | 2026-04-17 23:52:11 | not run |
| VB-CALL-036 | Which number did 923000001006 call the most with outgoing voice calls? | 923000001038 (before 923000001001, 923000001021, 923000001026) - 32 | not run |
| VB-CALL-037 | How many different numbers did 923000001006 contact, leaving out services? | 20 | not run |
| VB-CALL-038 | How many voice calls does 923000001010 have in this case? | 372 | not run |
| VB-CALL-039 | How many outgoing voice calls did 923000001010 make? | 215 | not run |
| VB-CALL-040 | How many incoming voice calls did 923000001010 receive? | 155 | not run |
| VB-CALL-041 | What is the total talk time of 923000001010 across all its voice calls, in seconds? | 41,672 seconds | not run |
| VB-CALL-042 | What is the duration of the longest voice call of 923000001010, in seconds? | 987 seconds | not run |
| VB-CALL-043 | What is the average duration of the voice calls of 923000001010, in seconds? | 112.02 seconds | not run |
| VB-CALL-044 | How many voice calls of 923000001010 lasted more than 10 minutes? | 1 | not run |
| VB-CALL-045 | How many voice calls of 923000001010 started between 00:00 and 04:59 Karachi time? | 10 | not run |
| VB-CALL-046 | When was the first call or message of 923000001010? | 2026-02-02 08:16:18 | not run |
| VB-CALL-047 | When was the last call or message of 923000001010? | 2026-04-17 17:30:21 | not run |
| VB-CALL-048 | Which number did 923000001010 call the most with outgoing voice calls? | 923000001080 (before 923000001052, 923000001093, 923000001097) - 56 | not run |
| VB-CALL-049 | How many different numbers did 923000001010 contact, leaving out services? | 15 | not run |
| VB-CALL-050 | How many voice calls does 923000001000 have in this case? | 261 | not run |
| VB-CALL-051 | How many outgoing voice calls did 923000001000 make? | 107 | not run |
| VB-CALL-052 | How many incoming voice calls did 923000001000 receive? | 154 | not run |
| VB-CALL-053 | What is the total talk time of 923000001000 across all its voice calls, in seconds? | 27,465 seconds | not run |
| VB-CALL-054 | What is the duration of the longest voice call of 923000001000, in seconds? | 1,102 seconds | not run |
| VB-CALL-055 | What is the average duration of the voice calls of 923000001000, in seconds? | 105.23 seconds | not run |
| VB-CALL-056 | How many voice calls of 923000001000 lasted more than 10 minutes? | 3 | not run |
| VB-CALL-057 | How many voice calls of 923000001000 started between 00:00 and 04:59 Karachi time? | 6 | not run |
| VB-CALL-058 | When was the first call or message of 923000001000? | 2026-02-02 07:38:45 | not run |
| VB-CALL-059 | When was the last call or message of 923000001000? | 2026-03-19 21:34:00 | not run |
| VB-CALL-060 | Which number did 923000001000 call the most with outgoing voice calls? | 923000001070 (before 923000001015, 923000001089, 923000001098) - 19 | not run |
| VB-CALL-061 | Which day had the most voice calls for 923000001000? | 2026-03-19 (before 2026-02-13, 2026-03-04) - 14 | not run |
| VB-CALL-062 | How many different numbers did 923000001000 contact, leaving out services? | 10 | not run |
| VB-CALL-063 | How many voice calls does 923000001007 have in this case? | 253 | not run |
| VB-CALL-064 | How many outgoing voice calls did 923000001007 make? | 119 | not run |
| VB-CALL-065 | How many incoming voice calls did 923000001007 receive? | 131 | not run |
| VB-CALL-066 | What is the total talk time of 923000001007 across all its voice calls, in seconds? | 23,487 seconds | not run |
| VB-CALL-067 | What is the duration of the longest voice call of 923000001007, in seconds? | 722 seconds | not run |
| VB-CALL-068 | What is the average duration of the voice calls of 923000001007, in seconds? | 92.83 seconds | not run |
| VB-CALL-069 | How many voice calls of 923000001007 lasted more than 10 minutes? | 2 | not run |
| VB-CALL-070 | How many voice calls of 923000001007 started between 00:00 and 04:59 Karachi time? | 3 | not run |
| VB-CALL-071 | When was the first call or message of 923000001007? | 2026-02-24 06:31:39 | not run |
| VB-CALL-072 | When was the last call or message of 923000001007? | 2026-04-17 22:20:58 | not run |
| VB-CALL-073 | Which number did 923000001007 call the most with outgoing voice calls? | 923000001057 (before 923000001073, 923000001087, 923000001072) - 29 | not run |
| VB-CALL-074 | Which day had the most voice calls for 923000001007? | 2026-03-18 (before 2026-03-11, 2026-02-24) - 11 | not run |
| VB-CALL-075 | How many different numbers did 923000001007 contact, leaving out services? | 8 | not run |
| VB-CALL-076 | How many voice calls does 923000001009 have in this case? | 342 | not run |
| VB-CALL-077 | How many outgoing voice calls did 923000001009 make? | 198 | not run |
| VB-CALL-078 | How many incoming voice calls did 923000001009 receive? | 144 | not run |
| VB-CALL-079 | What is the total talk time of 923000001009 across all its voice calls, in seconds? | 39,991 seconds | not run |
| VB-CALL-080 | What is the duration of the longest voice call of 923000001009, in seconds? | 1,800 seconds | not run |
| VB-CALL-081 | What is the average duration of the voice calls of 923000001009, in seconds? | 116.93 seconds | not run |
| VB-CALL-082 | How many voice calls of 923000001009 lasted more than 10 minutes? | 3 | not run |
| VB-CALL-083 | How many voice calls of 923000001009 started between 00:00 and 04:59 Karachi time? | 19 | not run |
| VB-CALL-084 | When was the first call or message of 923000001009? | 2026-02-02 09:25:19 | not run |
| VB-CALL-085 | When was the last call or message of 923000001009? | 2026-04-17 21:04:48 | not run |
| VB-CALL-086 | Which number did 923000001009 call the most with outgoing voice calls? | 923000001053 (before 923000001044, 923000001066, 923000001024) - 38 | not run |
| VB-CALL-087 | How many different numbers did 923000001009 contact, leaving out services? | 16 | not run |
| VB-CALL-088 | How many voice calls does 923000001001 have in this case? | 326 | not run |
| VB-CALL-089 | How many outgoing voice calls did 923000001001 make? | 146 | not run |
| VB-CALL-090 | How many incoming voice calls did 923000001001 receive? | 180 | not run |
| VB-CALL-091 | What is the total talk time of 923000001001 across all its voice calls, in seconds? | 31,663 seconds | not run |
| VB-CALL-092 | What is the duration of the longest voice call of 923000001001, in seconds? | 733 seconds | not run |
| VB-CALL-093 | What is the average duration of the voice calls of 923000001001, in seconds? | 97.13 seconds | not run |
| VB-CALL-094 | How many voice calls of 923000001001 lasted more than 10 minutes? | 1 | not run |
| VB-CALL-095 | How many voice calls of 923000001001 started between 00:00 and 04:59 Karachi time? | 15 | not run |
| VB-CALL-096 | When was the first call or message of 923000001001? | 2026-02-02 05:23:53 | not run |
| VB-CALL-097 | When was the last call or message of 923000001001? | 2026-04-17 20:05:51 | not run |
| VB-CALL-098 | Which number did 923000001001 call the most with outgoing voice calls? | 923000001078 (before 923000001038, 923000001006, 923000001069) - 33 | not run |
| VB-CALL-099 | Which day had the most voice calls for 923000001001? | 2026-02-05 (before 2026-02-25, 2026-02-16) - 11 | not run |
| VB-CALL-100 | How many different numbers did 923000001001 contact, leaving out services? | 10 | not run |
| VB-CALL-101 | How many voice calls does 923000001008 have in this case? | 206 | not run |
| VB-CALL-102 | How many outgoing voice calls did 923000001008 make? | 109 | not run |
| VB-CALL-103 | How many incoming voice calls did 923000001008 receive? | 97 | not run |
| VB-CALL-104 | What is the total talk time of 923000001008 across all its voice calls, in seconds? | 18,793 seconds | not run |
| VB-CALL-105 | What is the duration of the longest voice call of 923000001008, in seconds? | 371 seconds | not run |
| VB-CALL-106 | What is the average duration of the voice calls of 923000001008, in seconds? | 91.23 seconds | not run |
| VB-CALL-107 | How many voice calls of 923000001008 lasted more than 10 minutes? | 0 | not run |
| VB-CALL-108 | How many voice calls of 923000001008 started between 00:00 and 04:59 Karachi time? | 7 | not run |
| VB-CALL-109 | When was the first call or message of 923000001008? | 2026-03-05 10:18:48 | not run |
| VB-CALL-110 | When was the last call or message of 923000001008? | 2026-04-17 22:50:59 | not run |
| VB-CALL-111 | How many different numbers did 923000001008 contact, leaving out services? | 8 | not run |
| VB-CALL-112 | How many voice calls does 923000001003 have in this case? | 190 | not run |
| VB-CALL-113 | How many outgoing voice calls did 923000001003 make? | 95 | not run |
| VB-CALL-114 | How many incoming voice calls did 923000001003 receive? | 93 | not run |
| VB-CALL-115 | What is the total talk time of 923000001003 across all its voice calls, in seconds? | 19,255 seconds | not run |
| VB-CALL-116 | What is the duration of the longest voice call of 923000001003, in seconds? | 775 seconds | not run |
| VB-CALL-117 | What is the average duration of the voice calls of 923000001003, in seconds? | 101.34 seconds | not run |
| VB-CALL-118 | How many voice calls of 923000001003 lasted more than 10 minutes? | 1 | not run |
| VB-CALL-119 | How many voice calls of 923000001003 started between 00:00 and 04:59 Karachi time? | 9 | not run |
| VB-CALL-120 | When was the first call or message of 923000001003? | 2026-03-13 04:44:09 | not run |
| VB-CALL-121 | When was the last call or message of 923000001003? | 2026-04-17 21:33:43 | not run |
| VB-CALL-122 | Which number did 923000001003 call the most with outgoing voice calls? | 923000001069 (before 923000001021, 923000001095, 923000001006) - 18 | not run |
| VB-CALL-123 | Which day had the most voice calls for 923000001003? | 2026-04-10 (before 2026-03-19, 2026-03-30) - 10 | not run |
| VB-CALL-124 | How many different numbers did 923000001003 contact, leaving out services? | 8 | not run |
| VB-CALL-125 | How many voice calls does 923000001011 have in this case? | 169 | not run |
| VB-CALL-126 | How many outgoing voice calls did 923000001011 make? | 92 | not run |
| VB-CALL-127 | How many incoming voice calls did 923000001011 receive? | 76 | not run |
| VB-CALL-128 | What is the total talk time of 923000001011 across all its voice calls, in seconds? | 17,913 seconds | not run |
| VB-CALL-129 | What is the duration of the longest voice call of 923000001011, in seconds? | 662 seconds | not run |
| VB-CALL-130 | What is the average duration of the voice calls of 923000001011, in seconds? | 105.99 seconds | not run |
| VB-CALL-131 | How many voice calls of 923000001011 lasted more than 10 minutes? | 1 | not run |
| VB-CALL-132 | How many voice calls of 923000001011 started between 00:00 and 04:59 Karachi time? | 4 | not run |
| VB-CALL-133 | When was the first call or message of 923000001011? | 2026-02-02 02:20:13 | not run |
| VB-CALL-134 | When was the last call or message of 923000001011? | 2026-03-09 19:53:38 | not run |
| VB-CALL-135 | Which number did 923000001011 call the most with outgoing voice calls? | 923000001021 (before 923000001069, 923000001001, 923000001006) - 17 | not run |
| VB-CALL-136 | How many different numbers did 923000001011 contact, leaving out services? | 8 | not run |
| VB-CALL-137 | How many voice calls does 923000001004 have in this case? | 148 | not run |
| VB-CALL-138 | How many outgoing voice calls did 923000001004 make? | 82 | not run |
| VB-CALL-139 | How many incoming voice calls did 923000001004 receive? | 66 | not run |
| VB-CALL-140 | What is the total talk time of 923000001004 across all its voice calls, in seconds? | 13,707 seconds | not run |
| VB-CALL-141 | What is the duration of the longest voice call of 923000001004, in seconds? | 545 seconds | not run |
| VB-CALL-142 | What is the average duration of the voice calls of 923000001004, in seconds? | 92.61 seconds | not run |
| VB-CALL-143 | How many voice calls of 923000001004 lasted more than 10 minutes? | 0 | not run |
| VB-CALL-144 | How many voice calls of 923000001004 started between 00:00 and 04:59 Karachi time? | 6 | not run |
| VB-CALL-145 | When was the first call or message of 923000001004? | 2026-02-02 07:37:42 | not run |
| VB-CALL-146 | When was the last call or message of 923000001004? | 2026-03-03 19:58:06 | not run |
| VB-CALL-147 | Which number did 923000001004 call the most with outgoing voice calls? | 923000001066 (before 923000001044, 923000001061, 923000001090) - 24 | not run |
| VB-CALL-148 | Which day had the most voice calls for 923000001004? | 2026-02-12 (before 2026-02-16, 2026-02-03) - 9 | not run |
| VB-CALL-149 | How many different numbers did 923000001004 contact, leaving out services? | 8 | not run |

## Text messages of one subscriber

How many text messages were sent and received, to whom most, and when the first was sent.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-MESS-001 | How many text messages does 923000001002 have in this case? | 213 | correct: N: 213. \| first row: n 213 |
| VB-MESS-002 | How many text messages did 923000001002 send? | 93 | not run |
| VB-MESS-003 | How many text messages did 923000001002 receive? | 118 | not run |
| VB-MESS-004 | Which number did 923000001002 send the most text messages to? | 923000001048 (before 923000001012, 923000001054, 923000001027) - 21 | not run |
| VB-MESS-005 | When was the first text message of 923000001002? | 2026-02-02 06:12:39 | not run |
| VB-MESS-006 | How many text messages does 923000001006 have in this case? | 203 | not run |
| VB-MESS-007 | How many text messages did 923000001006 send? | 103 | not run |
| VB-MESS-008 | How many text messages did 923000001006 receive? | 99 | not run |
| VB-MESS-009 | Which number did 923000001006 send the most text messages to? | 923000001038 (before 923000001031, 923000001003, 923000001021) - 16 | not run |
| VB-MESS-010 | When was the first text message of 923000001006? | 2026-02-02 12:40:07 | not run |
| VB-MESS-011 | How many text messages does 923000001010 have in this case? | 172 | not run |
| VB-MESS-012 | How many text messages did 923000001010 send? | 97 | not run |
| VB-MESS-013 | How many text messages did 923000001010 receive? | 74 | not run |
| VB-MESS-014 | Which number did 923000001010 send the most text messages to? | 923000001052 (before 923000001093, 923000001080, 923000001074) - 22 | not run |
| VB-MESS-015 | When was the first text message of 923000001010? | 2026-02-02 12:34:45 | not run |
| VB-MESS-016 | How many text messages does 923000001000 have in this case? | 138 | not run |
| VB-MESS-017 | How many text messages did 923000001000 send? | 52 | not run |
| VB-MESS-018 | How many text messages did 923000001000 receive? | 86 | not run |
| VB-MESS-019 | Which number did 923000001000 send the most text messages to? | 923000001015 (before 923000001089, 923000001080, 923000001098) - 11 | not run |
| VB-MESS-020 | When was the first text message of 923000001000? | 2026-02-02 07:38:45 | not run |
| VB-MESS-021 | How many text messages does 923000001003 have in this case? | 110 | not run |
| VB-MESS-022 | How many text messages did 923000001003 send? | 56 | not run |
| VB-MESS-023 | How many text messages did 923000001003 receive? | 53 | not run |
| VB-MESS-024 | Which number did 923000001003 send the most text messages to? | 923000001069 (before 923000001006, 923000001031, 923000001001) - 14 | not run |
| VB-MESS-025 | When was the first text message of 923000001003? | 2026-03-13 07:52:14 | not run |
| VB-MESS-026 | How many text messages does 923000001007 have in this case? | 108 | not run |
| VB-MESS-027 | How many text messages did 923000001007 send? | 61 | not run |
| VB-MESS-028 | How many text messages did 923000001007 receive? | 47 | not run |
| VB-MESS-029 | Which number did 923000001007 send the most text messages to? | 923000001057 (before 923000001017, 923000001087, 923000001054) - 17 | not run |
| VB-MESS-030 | When was the first text message of 923000001007? | 2026-02-24 06:31:39 | not run |

## A subscriber and one other number

How many calls and messages two numbers had, how many outgoing calls, the total duration, and when they last had contact.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-PAIR-001 | How many calls and messages did 923000001005 have with 923000001036? | 150 | not run |
| VB-PAIR-002 | How many outgoing voice calls did 923000001005 make to 923000001036? | 105 | not run |
| VB-PAIR-003 | What is the total duration of the voice calls between 923000001005 and 923000001036, in seconds? | 14,673 seconds | not run |
| VB-PAIR-004 | When was the last contact between 923000001005 and 923000001036? | 2026-04-17 19:37:12 | not run |
| VB-PAIR-005 | How many calls and messages did 923000001002 have with 923000001027? | 146 | not run |
| VB-PAIR-006 | How many outgoing voice calls did 923000001002 make to 923000001027? | 25 | not run |
| VB-PAIR-007 | What is the total duration of the voice calls between 923000001002 and 923000001027, in seconds? | 12,290 seconds | not run |
| VB-PAIR-008 | When was the last contact between 923000001002 and 923000001027? | 2026-04-17 17:15:59 | not run |
| VB-PAIR-009 | How many calls and messages did 923000001006 have with 923000001038? | 93 | not run |
| VB-PAIR-010 | How many outgoing voice calls did 923000001006 make to 923000001038? | 32 | not run |
| VB-PAIR-011 | What is the total duration of the voice calls between 923000001006 and 923000001038, in seconds? | 6,183 seconds | not run |
| VB-PAIR-012 | When was the last contact between 923000001006 and 923000001038? | 2026-04-17 16:58:18 | not run |
| VB-PAIR-013 | How many calls and messages did 923000001010 have with 923000001080? | 124 | not run |
| VB-PAIR-014 | How many outgoing voice calls did 923000001010 make to 923000001080? | 56 | not run |
| VB-PAIR-015 | What is the total duration of the voice calls between 923000001010 and 923000001080, in seconds? | 8,512 seconds | not run |
| VB-PAIR-016 | When was the last contact between 923000001010 and 923000001080? | 2026-04-16 20:42:35 | not run |
| VB-PAIR-017 | How many calls and messages did 923000001000 have with 923000001098? | 75 | not run |
| VB-PAIR-018 | How many outgoing voice calls did 923000001000 make to 923000001098? | 14 | not run |
| VB-PAIR-019 | What is the total duration of the voice calls between 923000001000 and 923000001098, in seconds? | 5,613 seconds | not run |
| VB-PAIR-020 | When was the last contact between 923000001000 and 923000001098? | 2026-03-19 21:34:00 | not run |
| VB-PAIR-021 | How many calls and messages did 923000001007 have with 923000001057? | 84 | not run |
| VB-PAIR-022 | How many outgoing voice calls did 923000001007 make to 923000001057? | 29 | not run |
| VB-PAIR-023 | What is the total duration of the voice calls between 923000001007 and 923000001057, in seconds? | 5,298 seconds | not run |
| VB-PAIR-024 | When was the last contact between 923000001007 and 923000001057? | 2026-04-17 20:26:35 | not run |
| VB-PAIR-025 | How many calls and messages did 923000001009 have with 923000001053? | 72 | not run |
| VB-PAIR-026 | How many outgoing voice calls did 923000001009 make to 923000001053? | 38 | not run |
| VB-PAIR-027 | What is the total duration of the voice calls between 923000001009 and 923000001053, in seconds? | 6,659 seconds | not run |
| VB-PAIR-028 | When was the last contact between 923000001009 and 923000001053? | 2026-04-17 21:04:48 | not run |
| VB-PAIR-029 | How many calls and messages did 923000001001 have with 923000001078? | 63 | not run |
| VB-PAIR-030 | How many outgoing voice calls did 923000001001 make to 923000001078? | 33 | not run |
| VB-PAIR-031 | What is the total duration of the voice calls between 923000001001 and 923000001078, in seconds? | 6,546 seconds | not run |
| VB-PAIR-032 | When was the last contact between 923000001001 and 923000001078? | 2026-04-15 16:19:31 | not run |

## Internet sessions

Sessions, bytes downloaded and uploaded, the number of different apps, the app and the host used most, and which number has the most sessions.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-INTE-001 | How many internet sessions does 923000001010 have? | 431 | not run |
| VB-INTE-002 | How many bytes did 923000001010 download in total? | 214,448,386 bytes | not run |
| VB-INTE-003 | How many bytes did 923000001010 upload in total? | 20,985,010 bytes | not run |
| VB-INTE-004 | How many different apps did 923000001010 use in its internet sessions? | 3 | not run |
| VB-INTE-005 | Which app did 923000001010 use the most in its internet sessions? | app_a (before app_b, app_c) - 220 | not run |
| VB-INTE-006 | How many internet sessions does 923000001006 have? | 430 | not run |
| VB-INTE-007 | How many bytes did 923000001006 download in total? | 208,354,669 bytes | not run |
| VB-INTE-008 | How many bytes did 923000001006 upload in total? | 21,137,121 bytes | not run |
| VB-INTE-009 | How many different apps did 923000001006 use in its internet sessions? | 3 | not run |
| VB-INTE-010 | Which app did 923000001006 use the most in its internet sessions? | app_a (before app_b, app_c) - 228 | not run |
| VB-INTE-011 | How many internet sessions does 923000001009 have? | 421 | not run |
| VB-INTE-012 | How many bytes did 923000001009 download in total? | 200,016,221 bytes | not run |
| VB-INTE-013 | How many bytes did 923000001009 upload in total? | 20,771,501 bytes | not run |
| VB-INTE-014 | How many different apps did 923000001009 use in its internet sessions? | 3 | not run |
| VB-INTE-015 | Which app did 923000001009 use the most in its internet sessions? | app_a (before app_c, app_b) - 213 | not run |
| VB-INTE-016 | How many internet sessions does 923000001002 have? | 399 | not run |
| VB-INTE-017 | How many bytes did 923000001002 download in total? | 194,375,025 bytes | not run |
| VB-INTE-018 | How many bytes did 923000001002 upload in total? | 20,577,123 bytes | not run |
| VB-INTE-019 | How many different apps did 923000001002 use in its internet sessions? | 3 | not run |
| VB-INTE-020 | Which app did 923000001002 use the most in its internet sessions? | app_a (before app_c, app_b) - 191 | not run |
| VB-INTE-021 | How many internet sessions does 923000001005 have? | 389 | not run |
| VB-INTE-022 | How many bytes did 923000001005 download in total? | 195,404,638 bytes | not run |
| VB-INTE-023 | How many bytes did 923000001005 upload in total? | 19,464,161 bytes | not run |
| VB-INTE-024 | How many different apps did 923000001005 use in its internet sessions? | 3 | not run |
| VB-INTE-025 | Which app did 923000001005 use the most in its internet sessions? | app_a (before app_c, app_b) - 192 | not run |
| VB-INTE-026 | How many internet sessions does 923000001001 have? | 384 | not run |
| VB-INTE-027 | How many bytes did 923000001001 download in total? | 196,829,437 bytes | not run |
| VB-INTE-028 | How many bytes did 923000001001 upload in total? | 20,237,131 bytes | not run |
| VB-INTE-029 | How many different apps did 923000001001 use in its internet sessions? | 3 | not run |
| VB-INTE-030 | Which app did 923000001001 use the most in its internet sessions? | app_a (before app_c, app_b) - 207 | not run |
| VB-INTE-031 | Which number has the most internet sessions? | 923000001010 | not run |

## App usage

Records of app use per number, the number of different apps and the one used most.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-APPS-001 | How many app usage records does 923000001001 have? | 390 | not run |
| VB-APPS-002 | How many different apps did 923000001001 use? | 3 | not run |
| VB-APPS-003 | Which app did 923000001001 use the most? | app_a (before app_c, app_b) - 210 | not run |
| VB-APPS-004 | How many app usage records does 923000001000 have? | 267 | not run |
| VB-APPS-005 | How many different apps did 923000001000 use? | 3 | not run |
| VB-APPS-006 | Which app did 923000001000 use the most? | app_a (before app_b, app_c) - 141 | not run |

## Bank and wallet accounts

Transactions of an account, the totals credited and debited, the largest debit, the last balance, the first and last day, the account paid the most, how many different accounts were paid, who sent money, debits above an amount.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-BANK-001 | How many transactions does the account PK00SYNT0000000000002010 have? | 53 | not run |
| VB-BANK-002 | What is the total amount credited to the account PK00SYNT0000000000002010? | 1,156,096.41 rupees | not run |
| VB-BANK-003 | What is the total amount debited from the account PK00SYNT0000000000002010? | 4,161,008.11 rupees | not run |
| VB-BANK-004 | What is the largest single debit of the account PK00SYNT0000000000002010? | 490,500 rupees | not run |
| VB-BANK-005 | What was the balance after the last transaction of the account PK00SYNT0000000000002010? | 3,005,088.30 rupees | not run |
| VB-BANK-006 | On which day was the first transaction of the account PK00SYNT0000000000002010? | 2026-02-05 | not run |
| VB-BANK-007 | On which day was the last transaction of the account PK00SYNT0000000000002010? | 2026-04-16 | not run |
| VB-BANK-008 | Who did the account PK00SYNT0000000000002010 pay the most money to? | PK00SYNT0000000000005009 (before PK00SYNT0000000000005002, PK00SYNT0000000000005011) - 1,312,676.90 | not run |
| VB-BANK-009 | How many different accounts or numbers did the account PK00SYNT0000000000002010 pay? | 12 | not run |
| VB-BANK-010 | How many debits of the account PK00SYNT0000000000002010 were above 245250 rupees? | 5 | not run |
| VB-BANK-011 | How many transactions does the account PK00SYNT0000000000002004 have? | 52 | not run |
| VB-BANK-012 | What is the total amount credited to the account PK00SYNT0000000000002004? | 1,771,898.97 rupees | not run |
| VB-BANK-013 | What is the total amount debited from the account PK00SYNT0000000000002004? | 4,186,972.88 rupees | not run |
| VB-BANK-014 | What is the largest single debit of the account PK00SYNT0000000000002004? | 494,500 rupees | not run |
| VB-BANK-015 | What was the balance after the last transaction of the account PK00SYNT0000000000002004? | 3,510,987.64 rupees | not run |
| VB-BANK-016 | On which day was the first transaction of the account PK00SYNT0000000000002004? | 2026-02-03 | not run |
| VB-BANK-017 | On which day was the last transaction of the account PK00SYNT0000000000002004? | 2026-04-17 | not run |
| VB-BANK-018 | Who did the account PK00SYNT0000000000002004 pay the most money to? | PK00SYNT0000000000005005 (before PK00SYNT0000000000005010, PK00SYNT0000000000005011) - 614,205.31 | not run |
| VB-BANK-019 | How many different accounts or numbers did the account PK00SYNT0000000000002004 pay? | 12 | not run |
| VB-BANK-020 | How many debits of the account PK00SYNT0000000000002004 were above 247250 rupees? | 6 | not run |
| VB-BANK-021 | How many transactions does the account PK00SYNT0000000000002003 have? | 46 | not run |
| VB-BANK-022 | What is the total amount credited to the account PK00SYNT0000000000002003? | 1,847,854.55 rupees | not run |
| VB-BANK-023 | What is the total amount debited from the account PK00SYNT0000000000002003? | 2,646,194.03 rupees | not run |
| VB-BANK-024 | What is the largest single debit of the account PK00SYNT0000000000002003? | 384,500 rupees | not run |
| VB-BANK-025 | What was the balance after the last transaction of the account PK00SYNT0000000000002003? | 5,221,660.52 rupees | not run |
| VB-BANK-026 | On which day was the first transaction of the account PK00SYNT0000000000002003? | 2026-02-05 | not run |
| VB-BANK-027 | On which day was the last transaction of the account PK00SYNT0000000000002003? | 2026-04-17 | not run |
| VB-BANK-028 | Who did the account PK00SYNT0000000000002003 pay the most money to? | PK00SYNT0000000000005003 (before PK00SYNT0000000000005001, PK00SYNT0000000000005010) - 587,069.43 | not run |
| VB-BANK-029 | How many different accounts or numbers did the account PK00SYNT0000000000002003 pay? | 11 | not run |
| VB-BANK-030 | How many debits of the account PK00SYNT0000000000002003 were above 192250 rupees? | 5 | not run |
| VB-BANK-031 | How many transactions does the account PK00SYNT0000000000002007 have? | 36 | not run |
| VB-BANK-032 | What is the total amount credited to the account PK00SYNT0000000000002007? | 677,352.01 rupees | not run |
| VB-BANK-033 | What is the total amount debited from the account PK00SYNT0000000000002007? | 854,570.31 rupees | not run |
| VB-BANK-034 | What is the largest single debit of the account PK00SYNT0000000000002007? | 87,170.53 rupees | not run |
| VB-BANK-035 | What was the balance after the last transaction of the account PK00SYNT0000000000002007? | 5,912,781.70 rupees | not run |
| VB-BANK-036 | On which day was the first transaction of the account PK00SYNT0000000000002007? | 2026-02-02 | not run |
| VB-BANK-037 | On which day was the last transaction of the account PK00SYNT0000000000002007? | 2026-04-17 | not run |
| VB-BANK-038 | Who did the account PK00SYNT0000000000002007 pay the most money to? | PK00SYNT0000000000005007 (before PK00SYNT0000000000005002, PK00SYNT0000000000005003) - 187,574.41 | not run |
| VB-BANK-039 | How many different accounts or numbers did the account PK00SYNT0000000000002007 pay? | 10 | not run |
| VB-BANK-040 | How many debits of the account PK00SYNT0000000000002007 were above 43585 rupees? | 10 | not run |
| VB-BANK-041 | How many transactions does the account PK00SYNT0000000000002005 have? | 36 | not run |
| VB-BANK-042 | What is the total amount credited to the account PK00SYNT0000000000002005? | 660,455.52 rupees | not run |
| VB-BANK-043 | What is the total amount debited from the account PK00SYNT0000000000002005? | 756,235.72 rupees | not run |
| VB-BANK-044 | What is the largest single debit of the account PK00SYNT0000000000002005? | 86,869.78 rupees | not run |
| VB-BANK-045 | What was the balance after the last transaction of the account PK00SYNT0000000000002005? | 5,934,219.80 rupees | not run |
| VB-BANK-046 | On which day was the first transaction of the account PK00SYNT0000000000002005? | 2026-02-05 | not run |
| VB-BANK-047 | On which day was the last transaction of the account PK00SYNT0000000000002005? | 2026-04-17 | not run |
| VB-BANK-048 | Who did the account PK00SYNT0000000000002005 pay the most money to? | PK00SYNT0000000000005002 (before PK00SYNT0000000000007002, PK00SYNT0000000000005008) - 244,370.39 | not run |
| VB-BANK-049 | How many different accounts or numbers did the account PK00SYNT0000000000002005 pay? | 13 | not run |
| VB-BANK-050 | Which accounts or numbers sent money to the account PK00SYNT0000000000002005? | PK00SYNT0000000000005001, PK00SYNT0000000000005002, PK00SYNT0000000000005004, PK00SYNT0000000000005007, PK00SYNT0000000000005008, PK00SYNT0000000000005009, PK00SYNT0000000000009002 | not run |
| VB-BANK-051 | How many debits of the account PK00SYNT0000000000002005 were above 43434 rupees? | 5 | not run |
| VB-BANK-052 | How many transactions does the account PK00SYNT0000000000002001 have? | 35 | not run |
| VB-BANK-053 | What is the total amount credited to the account PK00SYNT0000000000002001? | 856,554.72 rupees | not run |
| VB-BANK-054 | What is the total amount debited from the account PK00SYNT0000000000002001? | 957,419.07 rupees | not run |
| VB-BANK-055 | What is the largest single debit of the account PK00SYNT0000000000002001? | 88,112.71 rupees | not run |
| VB-BANK-056 | What was the balance after the last transaction of the account PK00SYNT0000000000002001? | 5,979,135.65 rupees | not run |
| VB-BANK-057 | On which day was the first transaction of the account PK00SYNT0000000000002001? | 2026-02-04 | not run |
| VB-BANK-058 | On which day was the last transaction of the account PK00SYNT0000000000002001? | 2026-04-16 | not run |
| VB-BANK-059 | Who did the account PK00SYNT0000000000002001 pay the most money to? | PK00SYNT0000000000005003 (before PK00SYNT0000000000005009, PK00SYNT0000000000005007) - 171,945.64 | not run |
| VB-BANK-060 | How many different accounts or numbers did the account PK00SYNT0000000000002001 pay? | 11 | not run |
| VB-BANK-061 | How many debits of the account PK00SYNT0000000000002001 were above 44056 rupees? | 12 | not run |
| VB-BANK-062 | How many transactions does the account PK00SYNT0000000000002009 have? | 35 | not run |
| VB-BANK-063 | What is the total amount credited to the account PK00SYNT0000000000002009? | 516,585.01 rupees | not run |
| VB-BANK-064 | What is the total amount debited from the account PK00SYNT0000000000002009? | 863,186.33 rupees | not run |
| VB-BANK-065 | What is the largest single debit of the account PK00SYNT0000000000002009? | 85,988.25 rupees | not run |
| VB-BANK-066 | What was the balance after the last transaction of the account PK00SYNT0000000000002009? | 5,693,398.68 rupees | not run |
| VB-BANK-067 | On which day was the first transaction of the account PK00SYNT0000000000002009? | 2026-02-05 | not run |
| VB-BANK-068 | On which day was the last transaction of the account PK00SYNT0000000000002009? | 2026-04-16 | not run |
| VB-BANK-069 | Who did the account PK00SYNT0000000000002009 pay the most money to? | PK00SYNT0000000000005008 (before PK00SYNT0000000000005009, PK00SYNT0000000000005006) - 160,317.61 | not run |
| VB-BANK-070 | How many different accounts or numbers did the account PK00SYNT0000000000002009 pay? | 11 | not run |
| VB-BANK-071 | How many debits of the account PK00SYNT0000000000002009 were above 42994 rupees? | 10 | not run |
| VB-BANK-072 | How many transactions does the account PK00SYNT0000000000002011 have? | 34 | not run |
| VB-BANK-073 | What is the total amount credited to the account PK00SYNT0000000000002011? | 630,905.10 rupees | not run |
| VB-BANK-074 | What is the total amount debited from the account PK00SYNT0000000000002011? | 636,579.84 rupees | not run |
| VB-BANK-075 | What is the largest single debit of the account PK00SYNT0000000000002011? | 87,403.32 rupees | not run |
| VB-BANK-076 | What was the balance after the last transaction of the account PK00SYNT0000000000002011? | 6,104,325.26 rupees | not run |
| VB-BANK-077 | On which day was the first transaction of the account PK00SYNT0000000000002011? | 2026-02-05 | not run |
| VB-BANK-078 | On which day was the last transaction of the account PK00SYNT0000000000002011? | 2026-04-17 | not run |
| VB-BANK-079 | Who did the account PK00SYNT0000000000002011 pay the most money to? | PK00SYNT0000000000005003 (before PK00SYNT0000000000005006, PK00SYNT0000000000005008) - 142,680.70 | not run |
| VB-BANK-080 | How many different accounts or numbers did the account PK00SYNT0000000000002011 pay? | 10 | not run |
| VB-BANK-081 | How many debits of the account PK00SYNT0000000000002011 were above 43701 rupees? | 7 | not run |
| VB-BANK-082 | How many transactions does the account PK00SYNT0000000000002008 have? | 32 | not run |
| VB-BANK-083 | What is the total amount credited to the account PK00SYNT0000000000002008? | 699,422.95 rupees | not run |
| VB-BANK-084 | What is the total amount debited from the account PK00SYNT0000000000002008? | 820,306.92 rupees | not run |
| VB-BANK-085 | What is the largest single debit of the account PK00SYNT0000000000002008? | 87,250.09 rupees | not run |
| VB-BANK-086 | What was the balance after the last transaction of the account PK00SYNT0000000000002008? | 5,939,116.03 rupees | not run |
| VB-BANK-087 | On which day was the first transaction of the account PK00SYNT0000000000002008? | 2026-02-02 | not run |
| VB-BANK-088 | On which day was the last transaction of the account PK00SYNT0000000000002008? | 2026-04-15 | not run |
| VB-BANK-089 | Who did the account PK00SYNT0000000000002008 pay the most money to? | PK00SYNT0000000000005002 (before PK00SYNT0000000000005007, PK00SYNT0000000000005000) - 152,336.17 | not run |
| VB-BANK-090 | How many different accounts or numbers did the account PK00SYNT0000000000002008 pay? | 11 | not run |
| VB-BANK-091 | Which accounts or numbers sent money to the account PK00SYNT0000000000002008? | PK00SYNT0000000000005001, PK00SYNT0000000000005005, PK00SYNT0000000000005006, PK00SYNT0000000000005008, PK00SYNT0000000000005009, PK00SYNT0000000000005011, PK00SYNT0000000000009002 | not run |
| VB-BANK-092 | How many debits of the account PK00SYNT0000000000002008 were above 43625 rupees? | 10 | not run |
| VB-BANK-093 | How many transactions does the account PK00SYNT0000000000002006 have? | 30 | not run |
| VB-BANK-094 | What is the total amount credited to the account PK00SYNT0000000000002006? | 797,897.72 rupees | not run |
| VB-BANK-095 | What is the total amount debited from the account PK00SYNT0000000000002006? | 483,705.65 rupees | not run |
| VB-BANK-096 | What is the largest single debit of the account PK00SYNT0000000000002006? | 71,619.85 rupees | not run |
| VB-BANK-097 | What was the balance after the last transaction of the account PK00SYNT0000000000002006? | 6,414,192.07 rupees | not run |
| VB-BANK-098 | On which day was the first transaction of the account PK00SYNT0000000000002006? | 2026-02-02 | not run |
| VB-BANK-099 | On which day was the last transaction of the account PK00SYNT0000000000002006? | 2026-04-17 | not run |
| VB-BANK-100 | Who did the account PK00SYNT0000000000002006 pay the most money to? | PK00SYNT0000000000005004 (before PK00SYNT0000000000005007, PK00SYNT0000000000005006) - 107,949.34 | not run |
| VB-BANK-101 | How many different accounts or numbers did the account PK00SYNT0000000000002006 pay? | 9 | not run |
| VB-BANK-102 | How many debits of the account PK00SYNT0000000000002006 were above 35809 rupees? | 8 | not run |
| VB-BANK-103 | How many transactions does the account PK00SYNT0000000000002002 have? | 28 | not run |
| VB-BANK-104 | What is the total amount credited to the account PK00SYNT0000000000002002? | 483,381.14 rupees | not run |
| VB-BANK-105 | What is the total amount debited from the account PK00SYNT0000000000002002? | 893,711.55 rupees | not run |
| VB-BANK-106 | What is the largest single debit of the account PK00SYNT0000000000002002? | 83,926.58 rupees | not run |
| VB-BANK-107 | What was the balance after the last transaction of the account PK00SYNT0000000000002002? | 5,659,669.59 rupees | not run |
| VB-BANK-108 | On which day was the first transaction of the account PK00SYNT0000000000002002? | 2026-02-02 | not run |
| VB-BANK-109 | On which day was the last transaction of the account PK00SYNT0000000000002002? | 2026-04-17 | not run |
| VB-BANK-110 | Who did the account PK00SYNT0000000000002002 pay the most money to? | PK00SYNT0000000000005001 (before PK00SYNT0000000000005006, PK00SYNT0000000000005004) - 146,549.29 | not run |
| VB-BANK-111 | How many different accounts or numbers did the account PK00SYNT0000000000002002 pay? | 11 | not run |
| VB-BANK-112 | Which accounts or numbers sent money to the account PK00SYNT0000000000002002? | PK00SYNT0000000000005005, PK00SYNT0000000000005007, PK00SYNT0000000000005010, PK00SYNT0000000000005011, PK00SYNT0000000000009002 | not run |
| VB-BANK-113 | How many debits of the account PK00SYNT0000000000002002 were above 41963 rupees? | 12 | not run |
| VB-BANK-114 | How many transactions does the account PK00SYNT0000000000002000 have? | 28 | not run |
| VB-BANK-115 | What is the total amount credited to the account PK00SYNT0000000000002000? | 683,990.80 rupees | not run |
| VB-BANK-116 | What is the total amount debited from the account PK00SYNT0000000000002000? | 595,024.02 rupees | not run |
| VB-BANK-117 | What is the largest single debit of the account PK00SYNT0000000000002000? | 89,254.51 rupees | not run |
| VB-BANK-118 | What was the balance after the last transaction of the account PK00SYNT0000000000002000? | 6,138,966.78 rupees | not run |
| VB-BANK-119 | On which day was the first transaction of the account PK00SYNT0000000000002000? | 2026-02-02 | not run |
| VB-BANK-120 | On which day was the last transaction of the account PK00SYNT0000000000002000? | 2026-04-09 | not run |
| VB-BANK-121 | Who did the account PK00SYNT0000000000002000 pay the most money to? | PK00SYNT0000000000005011 (before PK00SYNT0000000000005008, PK00SYNT0000000000005004) - 142,095.46 | not run |
| VB-BANK-122 | How many different accounts or numbers did the account PK00SYNT0000000000002000 pay? | 9 | not run |
| VB-BANK-123 | Which accounts or numbers sent money to the account PK00SYNT0000000000002000? | PK00SYNT0000000000005000, PK00SYNT0000000000005001, PK00SYNT0000000000005003, PK00SYNT0000000000005006, PK00SYNT0000000000005009, PK00SYNT0000000000005011, PK00SYNT0000000000009002 | not run |
| VB-BANK-124 | How many debits of the account PK00SYNT0000000000002000 were above 44627 rupees? | 6 | not run |

## Vehicle plates and cameras

How often a plate was seen, at which cameras, when first and last; how many sightings a camera recorded and how many different plates; the plate seen the most.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-PLAT-001 | How many times was the plate SYNT2905 seen? | 50 | not run |
| VB-PLAT-002 | At which cameras was the plate SYNT2905 seen? | CAM-02, CAM-03, CAM-04, CAM-05, CAM-08 | not run |
| VB-PLAT-003 | When was the plate SYNT2905 first seen? | 2026-02-11 17:24:19 | not run |
| VB-PLAT-004 | When was the plate SYNT2905 last seen? | 2026-04-15 07:35:42 | not run |
| VB-PLAT-005 | How many times was the plate SYNT4024 seen? | 50 | not run |
| VB-PLAT-006 | At which cameras was the plate SYNT4024 seen? | CAM-01, CAM-03, CAM-05, CAM-07, CAM-08 | not run |
| VB-PLAT-007 | When was the plate SYNT4024 first seen? | 2026-02-16 23:44:49 | not run |
| VB-PLAT-008 | When was the plate SYNT4024 last seen? | 2026-04-09 00:44:37 | not run |
| VB-PLAT-009 | How many times was the plate SYNT2019 seen? | 30 | not run |
| VB-PLAT-010 | At which cameras was the plate SYNT2019 seen? | CAM-02, CAM-03, CAM-04, CAM-05, CAM-08 | not run |
| VB-PLAT-011 | When was the plate SYNT2019 first seen? | 2026-02-26 14:55:15 | not run |
| VB-PLAT-012 | When was the plate SYNT2019 last seen? | 2026-03-31 19:27:56 | not run |
| VB-PLAT-013 | How many times was the plate SYNT2233 seen? | 24 | not run |
| VB-PLAT-014 | At which cameras was the plate SYNT2233 seen? | CAM-01, CAM-05, CAM-06, CAM-08 | not run |
| VB-PLAT-015 | When was the plate SYNT2233 first seen? | 2026-02-03 17:21:55 | not run |
| VB-PLAT-016 | When was the plate SYNT2233 last seen? | 2026-04-14 12:29:14 | not run |
| VB-PLAT-017 | How many times was the plate SYNT9150 seen? | 14 | not run |
| VB-PLAT-018 | At which cameras was the plate SYNT9150 seen? | CAM-01, CAM-02 | not run |
| VB-PLAT-019 | When was the plate SYNT9150 first seen? | 2026-02-05 21:14:18 | not run |
| VB-PLAT-020 | When was the plate SYNT9150 last seen? | 2026-04-12 14:08:44 | not run |
| VB-PLAT-021 | How many times was the plate SYNT6147 seen? | 12 | not run |
| VB-PLAT-022 | At which cameras was the plate SYNT6147 seen? | CAM-05, CAM-07 | not run |
| VB-PLAT-023 | When was the plate SYNT6147 first seen? | 2026-03-14 20:47:27 | not run |
| VB-PLAT-024 | When was the plate SYNT6147 last seen? | 2026-04-15 17:27:08 | not run |
| VB-PLAT-025 | How many times was the plate SYNT4O24 seen? | 4 | not run |
| VB-PLAT-026 | At which cameras was the plate SYNT4O24 seen? | CAM-01, CAM-08, CAM-09 | not run |
| VB-PLAT-027 | When was the plate SYNT4O24 first seen? | 2026-02-04 15:53:32 | not run |
| VB-PLAT-028 | When was the plate SYNT4O24 last seen? | 2026-04-08 07:39:31 | not run |
| VB-PLAT-029 | How many times was the plate SYNT4016 seen? | 4 | not run |
| VB-PLAT-030 | At which cameras was the plate SYNT4016 seen? | CAM-03, CAM-04, CAM-08 | not run |
| VB-PLAT-031 | When was the plate SYNT4016 first seen? | 2026-03-02 00:58:25 | not run |
| VB-PLAT-032 | When was the plate SYNT4016 last seen? | 2026-03-17 23:44:36 | not run |
| VB-PLAT-033 | How many plate sightings did the camera CAM-01 record? | 36 | not run |
| VB-PLAT-034 | How many different plates did the camera CAM-01 see? | 16 | not run |
| VB-PLAT-035 | How many plate sightings did the camera CAM-02 record? | 31 | not run |
| VB-PLAT-036 | How many different plates did the camera CAM-02 see? | 9 | not run |
| VB-PLAT-037 | How many plate sightings did the camera CAM-03 record? | 33 | not run |
| VB-PLAT-038 | How many different plates did the camera CAM-03 see? | 9 | not run |
| VB-PLAT-039 | How many plate sightings did the camera CAM-04 record? | 33 | not run |
| VB-PLAT-040 | How many different plates did the camera CAM-04 see? | 16 | not run |
| VB-PLAT-041 | How many plate sightings did the camera CAM-05 record? | 49 | not run |
| VB-PLAT-042 | How many different plates did the camera CAM-05 see? | 16 | not run |
| VB-PLAT-043 | How many plate sightings did the camera CAM-06 record? | 16 | not run |
| VB-PLAT-044 | How many different plates did the camera CAM-06 see? | 9 | not run |
| VB-PLAT-045 | How many plate sightings did the camera CAM-07 record? | 33 | not run |
| VB-PLAT-046 | How many different plates did the camera CAM-07 see? | 16 | not run |
| VB-PLAT-047 | How many plate sightings did the camera CAM-08 record? | 45 | not run |
| VB-PLAT-048 | How many different plates did the camera CAM-08 see? | 15 | not run |
| VB-PLAT-049 | How many plate sightings did the camera CAM-09 record? | 10 | not run |
| VB-PLAT-050 | How many different plates did the camera CAM-09 see? | 10 | not run |
| VB-PLAT-051 | Which plate was seen the most? | SYNT2905 (before SYNT4024) - 50 | not run |

## Handsets

Which handset (IMEI) a number was seen with and with how many.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-HAND-001 | Which handset (IMEI) was 923000001005 seen with? | 000000000010017 | not run |
| VB-HAND-002 | How many different handsets was 923000001005 seen with? | 1 | not run |
| VB-HAND-003 | Which handset (IMEI) was 923000001002 seen with? | 000000000010025, 000000000200014 | not run |
| VB-HAND-004 | How many different handsets was 923000001002 seen with? | 2 | not run |
| VB-HAND-005 | Which handset (IMEI) was 923000001006 seen with? | 000000000010066, 000000000200048 | not run |
| VB-HAND-006 | How many different handsets was 923000001006 seen with? | 2 | not run |
| VB-HAND-007 | Which handset (IMEI) was 923000001010 seen with? | 000000000010108 | not run |
| VB-HAND-008 | How many different handsets was 923000001010 seen with? | 1 | not run |
| VB-HAND-009 | Which handset (IMEI) was 923000001000 seen with? | 000000000010009 | not run |
| VB-HAND-010 | How many different handsets was 923000001000 seen with? | 1 | not run |
| VB-HAND-011 | Which handset (IMEI) was 923000001007 seen with? | 00000000001007 | not run |
| VB-HAND-012 | How many different handsets was 923000001007 seen with? | 1 | not run |
| VB-HAND-013 | Which handset (IMEI) was 923000001009 seen with? | 000000000010090 | not run |
| VB-HAND-014 | How many different handsets was 923000001009 seen with? | 1 | not run |
| VB-HAND-015 | Which handset (IMEI) was 923000001001 seen with? | 000000000010017 | not run |
| VB-HAND-016 | How many different handsets was 923000001001 seen with? | 1 | not run |
| VB-HAND-017 | Which handset (IMEI) was 923000001008 seen with? | 000000000010041 | not run |
| VB-HAND-018 | How many different handsets was 923000001008 seen with? | 1 | not run |
| VB-HAND-019 | Which handset (IMEI) was 923000001003 seen with? | 00000000001003 | not run |
| VB-HAND-020 | How many different handsets was 923000001003 seen with? | 1 | not run |
| VB-HAND-021 | Which handset (IMEI) was 923000001011 seen with? | 00000000001011 | not run |
| VB-HAND-022 | How many different handsets was 923000001011 seen with? | 1 | not run |
| VB-HAND-023 | Which handset (IMEI) was 923000001004 seen with? | 000000000010041 | not run |
| VB-HAND-024 | How many different handsets was 923000001004 seen with? | 1 | not run |

## Cell sites

The cell site a number used most and how many different cell sites it used. A cell site is a coverage area, not a position.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-CELL-001 | Which cell site did 923000001005 use the most? | ?:10221:10629 (before ?:10273:10777, ?:10234:10666) - 338 | not run |
| VB-CELL-002 | How many different cell sites did 923000001005 use? | 6 | not run |
| VB-CELL-003 | Which cell site did 923000001002 use the most? | ?:?:3003665515 (before ?:?:3003141870, ?:?:3003246599) - 362 | not run |
| VB-CELL-004 | How many different cell sites did 923000001002 use? | 5 | not run |
| VB-CELL-005 | Which cell site did 923000001006 use the most? | ?:?:3003456057 (before ?:?:3002513496, ?:?:3003665515) - 307 | not run |
| VB-CELL-006 | How many different cell sites did 923000001006 use? | 5 | not run |
| VB-CELL-007 | Which cell site did 923000001010 use the most? | ?:?:3003665515 (before ?:?:3002513496, ?:?:3002618225) - 274 | not run |
| VB-CELL-008 | How many different cell sites did 923000001010 use? | 5 | not run |
| VB-CELL-009 | Which cell site did 923000001000 use the most? | ?:?:103d13e (before ?:?:108i18j, ?:?:104e14f) - 203 | not run |
| VB-CELL-010 | How many different cell sites did 923000001000 use? | 4 | not run |
| VB-CELL-011 | Which cell site did 923000001007 use the most? | ?:518:10300922 (before ?:606:10364274, ?:617:10372193) - 195 | not run |
| VB-CELL-012 | How many different cell sites did 923000001007 use? | 4 | not run |
| VB-CELL-013 | Which cell site did 923000001009 use the most? | ?:10156:10444 (before ?:10286:10814, ?:10169:10481) - 164 | not run |
| VB-CELL-014 | How many different cell sites did 923000001009 use? | 6 | not run |
| VB-CELL-015 | Which cell site did 923000001001 use the most? | ?:10286:10814 (before ?:10221:10629, ?:10299:10851) - 146 | not run |
| VB-CELL-016 | How many different cell sites did 923000001001 use? | 5 | not run |

## Registrations

Which SIM identity a number is registered with and the status of the registration. A registration is what a source file states, not who holds the number.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-REGI-001 | Which SIM identity (IMSI) is registered for the number 923000001004? | 001010000001004 | not run |
| VB-REGI-002 | What is the status of the registration of the number 923000001004? | active | not run |
| VB-REGI-003 | Which SIM identity (IMSI) is registered for the number 923000001007? | 001010000001007 | not run |
| VB-REGI-004 | What is the status of the registration of the number 923000001007? | deactivated | not run |
| VB-REGI-005 | Which SIM identity (IMSI) is registered for the number 923000001009? | 001010000001009 | not run |
| VB-REGI-006 | What is the status of the registration of the number 923000001009? | deactivated | not run |
| VB-REGI-007 | Which SIM identity (IMSI) is registered for the number 923000001010? | 001010000001010 | not run |
| VB-REGI-008 | What is the status of the registration of the number 923000001010? | deactivated | not run |
| VB-REGI-009 | Which SIM identity (IMSI) is registered for the number 923000001012? | 001010000001012 | not run |
| VB-REGI-010 | What is the status of the registration of the number 923000001012? | active | not run |
| VB-REGI-011 | Which SIM identity (IMSI) is registered for the number 923000001013? | 001010000001013 | not run |
| VB-REGI-012 | What is the status of the registration of the number 923000001013? | active | not run |
| VB-REGI-013 | Which SIM identity (IMSI) is registered for the number 923000001014? | 001010000001014 | not run |
| VB-REGI-014 | What is the status of the registration of the number 923000001014? | active | not run |
| VB-REGI-015 | Which SIM identity (IMSI) is registered for the number 923000001015? | 001010000001015 | not run |
| VB-REGI-016 | What is the status of the registration of the number 923000001015? | unknown | not run |

## The evidence files

How many files the case holds, in total and by type.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| VB-INVE-001 | How many document files are in this case? | 4 | not run |
| VB-INVE-002 | How many structured record files are in this case? | 42 | not run |
| VB-INVE-003 | How many tabular files are in this case? | 3 | not run |
| VB-INVE-004 | How many text files are in this case? | 4 | not run |
| VB-INVE-005 | How many evidence files are in this case? | 53 | not run |

