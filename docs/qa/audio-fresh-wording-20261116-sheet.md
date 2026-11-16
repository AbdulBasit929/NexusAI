# Verified question bank: the Orion Lahore reference case

34 questions, each with the answer computed from the stored rows or readings (never through the engine) and the live engine's answer scored against it. Synthetic data only.

| Family | Questions | Correct | Wrong | Right number, wrong view | Declined or abstained | Not run |
|---|---|---|---|---|---|---|
| Speech recordings (Pashto, Urdu, English) | 34 | 27 | 1 | 0 | 6 | 0 |
| **All** | **34** | **27** | **1** | **0** | **6** | **0** |

## Speech recordings (Pashto, Urdu, English)

The ten audio recordings of the case, public speech of real speakers: how many there are and how long, which are in Pashto, Urdu or English, how many segments a recording has, which recordings say a word, and what a recording says. The text is a machine transcript; Pashto is a search lead, not a transcript.

| ID | Question | Expected | Live engine |
|---|---|---|---|
| FA-001 | What is the duration of voice-note-07.wav? | 6.48 seconds | correct: Duration seconds: 6.48. \| first row: duration_seconds 6.48 |
| FA-002 | Tell me how many seconds voice-note-09.wav lasts. | 8.80 seconds | abstained: I did not give an answer, because I could not show that the query I wrote asks everything you asked: the quest |
| FA-003 | How many seconds is the file voice-note-01.wav? | 9.72 seconds | abstained: I did not give an answer, because I could not show that the query I wrote asks everything you asked: the quest |
| FA-004 | Which recording has the shortest duration? | voice-note-03.wav | correct: 1 result: voice-note-03.wav (6). \| first row: label voice-note-03.wav; value 6 |
| FA-005 | What is the combined running time of every audio file in the case, in minutes? | 1.43 minutes | wrong: Minutes: 1.4. \| first row: minutes 1.4 |
| FA-006 | How many recordings run longer than 9.5 seconds? | 5 | correct: Count: 5. \| first row: count 5 |
| FA-007 | List the audio files that are under 8 seconds. | voice-note-03.wav, voice-note-06.wav, voice-note-07.wav, voice-note-10.wav | correct: 4 rows. \| first row: duration_seconds 7.79; original_filename voice-note-10.wav |
| FA-008 | What is the average length of the audio recordings, in seconds? | 8.59 seconds | correct: Average length seconds: 8.593. \| first row: average_length_seconds 8.593 |
| FA-009 | Which voice notes are spoken in English? | voice-note-03.wav, voice-note-06.wav, voice-note-09.wav | correct: 3 rows. \| first row: original_filename voice-note-03.wav |
| FA-010 | List the recordings where Urdu is spoken. | voice-note-01.wav, voice-note-04.wav, voice-note-07.wav | declined: No observation or supported adjacent-segment window matched the supplied text filter in the searched current s |
| FA-011 | Count the recordings whose language is Pashto. | 4 | correct: A reading a model made from a photo or a video frame is a candidate that needs checking. It does not say who o |
| FA-012 | Which language is spoken in voice-note-07.wav? | one of: Urdu, ur; and not: Pashto, English, Hindi, Punjabi | correct: Language name: Urdu. \| first row: language_name Urdu |
| FA-013 | Is voice-note-04.wav in Pashto, Urdu or English? | one of: Urdu, ur | correct: Language name: Urdu. \| first row: language_name Urdu |
| FA-014 | How many different languages are spoken across the recordings? | 3 | correct: N: 3. \| first row: n 3 |
| FA-015 | Which language has the most recordings? | Pashto | correct: 1 result: Pashto (4). \| first row: label Pashto; value 4 |
| FA-016 | Which recordings are in Urdu and longer than 9.2 seconds? | voice-note-01.wav, voice-note-04.wav | correct: 2 rows. \| first row: duration_seconds 10.56; original_filename voice-note-04.wav |
| FA-017 | What is the longest English recording? | voice-note-09.wav | declined: The text search is incomplete. Any listed observations are supported matches; absence of further matches has n |
| FA-018 | How long is the shortest Pashto recording, in seconds? | 7.79 seconds | declined: The text search is incomplete. Any listed observations are supported matches; absence of further matches has n |
| FA-019 | How many speech segments do the Urdu recordings have in total? | 5 | correct: N: 5. \| first row: n 5 |
| FA-020 | How many segments is voice-note-04.wav split into? | 3 | correct: N: 3. \| first row: n 3 |
| FA-021 | Into how many speech segments was voice-note-09.wav divided? | 1 | correct: N: 1. \| first row: n 1 |
| FA-022 | What is the total number of speech segments across all the audio files? | 12 | correct: N: 12. \| first row: n 12 |
| FA-023 | Which recording has the most speech segments? | voice-note-04.wav | correct: 1 result: voice-note-04.wav (3). \| first row: label voice-note-04.wav; value 3 |
| FA-024 | How many recordings consist of a single segment? | 9 | correct: N: 9. \| first row: n 9 |
| FA-025 | Which audio files still need a person to check them? | voice-note-01.wav, voice-note-02.wav, voice-note-03.wav, voice-note-04.wav, voice-note-05.wav, voice-note-06.wav, voice-note-07.wav, voice-note-08.wav, voice-note-09.wav, voice-note-10.wav | correct: 10 rows. \| first row: original_filename voice-note-01.wav |
| FA-026 | How many recordings need manual review? | 10 | correct: N: 10. \| first row: n 10 |
| FA-027 | Show the text of voice-note-06.wav | all of: Along, line, wear | declined: I could not map that request to a deterministic forensic workflow. Please name the evidence family and the cal |
| FA-028 | Transcribe voice-note-07.wav | all of: ڈائنسور, نہیں, کیونکہ | correct: Text: بہت سے لوگ ان کو ڈائنسور تسلیم نہیں کرتے کیونکہ وہ بال وپر رکھتے ہیں اور اڑھنے کے قابل تھے. \| first row: |
| FA-029 | What does the speaker in voice-note-08.wav say? | all of: هیواد, خارونو, بیلوالي | correct: Start seconds 0; End seconds 10.02; Text دا ښار د هیواد نورو خارونو سره ډیر ښکلې بیلوالي لري ځکه چې دا د افریک |
| FA-030 | Which of the voice notes talk about sharpener? | voice-note-09.wav | correct: voice-note-09.wav mentions it at 0 seconds: “People now write messages on computer screens never having to com |
| FA-031 | Search the recordings for تسلیم | voice-note-07.wav | correct: voice-note-07.wav mentions it at 0 seconds: “بہت سے لوگ ان کو ڈائنسور تسلیم نہیں کرتے کیونکہ وہ بال وپر رکھتے  |
| FA-032 | Where is انسان mentioned in the audio? | voice-note-10.wav | correct: voice-note-10.wav mentions it at 0 seconds: “داسې څاروای خو ذهن ته نه راځي خو دا چلند کولی شي انسان یا هیوان و |
| FA-033 | Which recording contains trousers? | voice-note-03.wav, voice-note-06.wav | correct: voice-note-06.wav mentions it at 0 seconds: “Along the same line, men are required to wear trousers covering t |
| FA-034 | Which audio recordings mention the word zebra? | none | correct: No observation or supported adjacent-segment window matched the supplied text filter in the searched current s |

