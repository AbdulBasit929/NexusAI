"""Which questions a run asks: the spread (--per-intent) is what it always was, and --intents narrows it first.

    python evaluation/question_factory/test_factory_select.py
"""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import factory as f  # noqa: E402


def item(family, intent, n):
    return {"id": "%s-%s-%02d" % (family.upper(), intent, n), "family": family, "intent": intent}


ITEMS = [
    item("cdr", "count_eq", 1), item("cdr", "count_eq", 2), item("cdr", "night", 1), item("cdr", "night", 2),
    item("ipdr", "count_eq", 1), item("ipdr", "night", 1), item("anpr", "earliest", 1), item("anpr", "earliest", 2),
    item("anpr", "top_group", 1),
]


def ids(items):
    return [i["id"] for i in items]


def test_with_nothing_asked_every_question_is_asked_in_file_order():
    assert ids(f.select_items(ITEMS)) == ids(ITEMS)


def test_per_intent_takes_the_first_of_each_family_and_intent_as_it_always_did():
    assert ids(f.select_items(ITEMS, per_intent=1)) == [
        "CDR-count_eq-01", "CDR-night-01", "IPDR-count_eq-01", "IPDR-night-01", "ANPR-earliest-01", "ANPR-top_group-01"]
    assert ids(f.select_items(ITEMS, per_intent=2))[:4] == ["CDR-count_eq-01", "CDR-count_eq-02", "CDR-night-01", "CDR-night-02"]


def test_intents_keeps_only_the_named_ones():
    assert ids(f.select_items(ITEMS, intents="night")) == ["CDR-night-01", "CDR-night-02", "IPDR-night-01"]
    assert ids(f.select_items(ITEMS, intents="night, earliest")) == [
        "CDR-night-01", "CDR-night-02", "IPDR-night-01", "ANPR-earliest-01", "ANPR-earliest-02"]


def test_intents_is_applied_before_the_spread_and_the_limit_last():
    assert ids(f.select_items(ITEMS, per_intent=1, intents="night,earliest")) == ["CDR-night-01", "IPDR-night-01", "ANPR-earliest-01"]
    assert ids(f.select_items(ITEMS, per_intent=1, intents="night,earliest", limit=2)) == ["CDR-night-01", "IPDR-night-01"]


def test_an_intent_no_question_has_gives_no_questions_and_blank_names_are_ignored():
    assert f.select_items(ITEMS, intents="nonexistent") == []
    assert ids(f.select_items(ITEMS, intents=" ,night,, ")) == ["CDR-night-01", "CDR-night-02", "IPDR-night-01"]


def test_the_original_list_is_not_changed():
    before = list(ITEMS)
    f.select_items(ITEMS, per_intent=1, intents="night", limit=1)
    assert ITEMS == before


if __name__ == "__main__":
    for name, fn in list(globals().items()):
        if name.startswith("test_"):
            fn()
            print("ok", name)
