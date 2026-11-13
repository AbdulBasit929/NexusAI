"""Tests of the pure parts of tools/media_data/omni_ctc.py (the SentencePiece reader and the key map); the model itself is checked on the real checkpoint by `convert`, which refuses a mismatch.

    python -I -X utf8 tools/media_data/test_omni_ctc.py
"""
import os
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)

import omni_ctc as oc  # noqa: E402


def varint(n):
    out = bytearray()
    while True:
        byte = n & 0x7F
        n >>= 7
        out.append(byte | (0x80 if n else 0))
        if not n:
            return bytes(out)


def piece(text, kind=None, score=b"\x00\x00\x80\xbf"):
    body = b"\x0a" + varint(len(text.encode())) + text.encode() + b"\x15" + score
    if kind is not None:
        body += b"\x18" + varint(kind)
    return b"\x0a" + varint(len(body)) + body


class Vocabulary(unittest.TestCase):
    def test_pieces_are_read_in_order_with_their_type_and_unrelated_fields_are_skipped(self):
        trainer = b"\x12" + varint(3) + b"abc"                     # field 2 (trainer spec) in the model message: must be skipped
        data = piece("<s>", 3) + trainer + piece("▁پښتو") + piece("a", 1)
        with tempfile.NamedTemporaryFile(delete=False) as handle:
            handle.write(data)
        try:
            pieces = oc.sentencepiece_pieces(handle.name)
        finally:
            os.remove(handle.name)
        self.assertEqual(pieces, [("<s>", 3), ("▁پښتو", 1), ("a", 1)])

    def test_a_long_piece_uses_a_multi_byte_length(self):
        text = "x" * 300
        with tempfile.NamedTemporaryFile(delete=False) as handle:
            handle.write(piece(text))
        try:
            self.assertEqual(oc.sentencepiece_pieces(handle.name), [(text, 1)])
        finally:
            os.remove(handle.name)


class KeyMap(unittest.TestCase):
    def test_every_layer_and_convolution_has_a_target_and_targets_are_unique(self):
        mapping = oc.key_map(24)
        self.assertEqual(len(set(mapping.values())), len(mapping))
        self.assertEqual(mapping["encoder.layers.23.ffn.inner_proj.weight"], "wav2vec2.encoder.layers.23.feed_forward.intermediate_dense.weight")
        self.assertEqual(mapping["encoder_frontend.feature_extractor.layers.6.layer_norm.bias"], "wav2vec2.feature_extractor.conv_layers.6.layer_norm.bias")
        self.assertEqual(mapping["final_proj.weight"], "lm_head.weight")
        self.assertEqual(len(mapping), len(oc.STATIC) + 7 * 4 + 24 * 16)


class Decoding(unittest.TestCase):
    def test_greedy_ctc_collapses_repeats_drops_blanks_and_turns_the_word_marker_into_a_space(self):
        decoder = oc.OmniCTC.__new__(oc.OmniCTC)
        decoder.pieces = ["<s>", "▁a", "b", "▁c", "</s>"]
        self.assertEqual(decoder.decode([0, 1, 1, 0, 2, 2, 3, 3, 0, 3, 4]), "ab c c")


class ScriptConstraint(unittest.TestCase):
    def test_only_arabic_script_letters_and_neutral_pieces_are_allowed(self):
        pieces = ["<s>", "▁", " ", "ا", "جز", "ज", "a", "5", ".", "▁है"]
        self.assertEqual(oc.allowed_pieces(pieces), [True, True, True, True, True, False, False, True, True, False])
        self.assertEqual(oc.allowed_pieces([chr(0x93e), chr(0x947) + chr(0x902), chr(0x64e) + chr(0x627)]), [False, False, True])      # Devanagari vowel signs are marks, not letters; an Arabic diacritic is allowed


if __name__ == "__main__":
    unittest.main()
