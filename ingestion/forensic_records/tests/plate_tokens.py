"""The plate tokens both normalisers are run on: normalize_plate (Python, layout_registry/parties.py) and forensic.normalize_plate (SQL, migration 012).

Used by test_layout_registry.PlateTests (Python alone) and by tools/case_generator/test_plate_views.py (the same tokens through the SQL function on a throwaway database; gate G2 of
docs/data/anpr-typed-family-20261026.md: zero differences). Every token is made of escapes, so that no confusable character hides in the source.
"""

PLATE_TOKENS = [
    "LEA-1234", "lea-1234", "LEA 1234", " lea  1234 ", "LEA.1234", "LEA_1234", "LEA/1234", "LEA1234", "ABC-123", "abc 123", "AB 12 3456", "A-B-C 1 2 3",
    "SYNT-0001", "synt 0001", "SYNT–0001", "SYNT—1", "SYNT−1", "SYNT·0001", "SYNT•0001", "SYNT‐0001", "SYNT‑0001", "SYNT‒0001", "SYNT―0001",
    "SYNT-٠٠٠١", "SYNT-۰۰۰۲", "ابج-١٢٣", "ابج ۱۲۳", "ا ب ج 1 2 3",
    "ＡＢＣ-１２３", "ＡＢＣ　１２３", "ABC 123", "ABC\t123", "ABC\n123", "ABC--123", "ABC - 123", "A.B.C-1.2.3",
    "AB0-O1", "ab0-o1", "AB0-01", "AB1-I1", "O0O0", "0O0O", "L1", "l1", "12345", "ABCDE", "abcde", "٠١", "-", ".", " ", "", "ABC­123", "①②", "ﬁx-12", "ÉCOLE-12",
    "اب-AB-12", "LEA-1234 ", "  SYNT-9999", "SYNT-0000", "xyz-9", "XYZ-90", "QQ-77",
]
