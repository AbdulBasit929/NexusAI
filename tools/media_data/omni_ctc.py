#!/usr/bin/env python3
"""Runs the Omnilingual ASR CTC checkpoints (Apache-2.0, Meta) on the CPU with the packages already in the evaluation image: torch and transformers' Wav2Vec2ForCTC. No fairseq2, no download.

    python omni_ctc.py convert  <omniASR-CTC-300M.pt> <omniASR_tokenizer.model> <output_dir>
    python omni_ctc.py describe <omniASR-CTC-300M.pt>

Why this exists: Meta's own route needs fairseq2 (not installable here without a new dependency) and the one public converter needs fairseq2 as well (and has no licence file, so it is not copied). The
checkpoint is a plain dictionary of tensors; the model is a wav2vec2 encoder with a CTC head, a standard architecture. This file maps the checkpoint's parameter names onto Wav2Vec2ForCTC, reads the
SentencePiece vocabulary (a protobuf file, parsed here by hand: pieces are field 1 of the model message and their text is field 1 of each piece) and decodes greedily. The architecture facts it relies on
(seven convolution layers with layer norm and bias, convolutional position embedding with kernel 128 and 16 groups, pre-norm transformer, 64 per head, instance normalisation of the waveform, CTC blank
= token 0) are written down in the model's published description; here they are CHECKED instead of trusted: every parameter of the HF model must be filled from exactly one checkpoint tensor of the same
shape (strict load) and no checkpoint tensor may be left over. Whether the converted model reads speech correctly is then measured (character error on a Pashto set), not assumed.
The checkpoint is loaded with torch.load(weights_only=True): a file that holds anything but tensors and plain containers is refused.
"""
import json
import os
import sys


# ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ the vocabulary


def read_varint(data, pos):
    shift = result = 0
    while True:
        byte = data[pos]
        pos += 1
        result |= (byte & 0x7F) << shift
        if not byte & 0x80:
            return result, pos
        shift += 7


def sentencepiece_pieces(path):
    """[(piece text, type)] of a SentencePiece model file, in id order. Message ModelProto: field 1 = repeated SentencePiece {1 piece (string), 2 score (float), 3 type (enum, default NORMAL=1)}."""
    with open(path, "rb") as handle:
        data = handle.read()
    pieces, pos = [], 0
    while pos < len(data):
        key, pos = read_varint(data, pos)
        field, wire = key >> 3, key & 7
        if wire == 0:
            _, pos = read_varint(data, pos)
        elif wire == 1:
            pos += 8
        elif wire == 5:
            pos += 4
        elif wire == 2:
            length, pos = read_varint(data, pos)
            body = data[pos:pos + length]
            pos += length
            if field == 1:
                text, kind, p = "", 1, 0
                while p < len(body):
                    k, p = read_varint(body, p)
                    f, w = k >> 3, k & 7
                    if w == 2:
                        n, p = read_varint(body, p)
                        if f == 1:
                            text = body[p:p + n].decode("utf-8")
                        p += n
                    elif w == 0:
                        v, p = read_varint(body, p)
                        if f == 3:
                            kind = v
                    elif w == 5:
                        p += 4
                    elif w == 1:
                        p += 8
                    else:
                        raise ValueError("unsupported wire type %d in a piece" % w)
                pieces.append((text, kind))
        else:
            raise ValueError("unsupported wire type %d" % wire)
    return pieces


# ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ the weights

STATIC = {
    "encoder_frontend.post_extract_layer_norm.weight": "wav2vec2.feature_projection.layer_norm.weight",
    "encoder_frontend.post_extract_layer_norm.bias": "wav2vec2.feature_projection.layer_norm.bias",
    "encoder_frontend.model_dim_proj.weight": "wav2vec2.feature_projection.projection.weight",
    "encoder_frontend.model_dim_proj.bias": "wav2vec2.feature_projection.projection.bias",
    "encoder_frontend.pos_encoder.conv.bias": "wav2vec2.encoder.pos_conv_embed.conv.bias",
    "encoder_frontend.pos_encoder.conv.weight_g": "wav2vec2.encoder.pos_conv_embed.conv.parametrizations.weight.original0",
    "encoder_frontend.pos_encoder.conv.weight_v": "wav2vec2.encoder.pos_conv_embed.conv.parametrizations.weight.original1",
    "encoder.layer_norm.weight": "wav2vec2.encoder.layer_norm.weight",
    "encoder.layer_norm.bias": "wav2vec2.encoder.layer_norm.bias",
    "final_proj.weight": "lm_head.weight",
    "final_proj.bias": "lm_head.bias",
}
LAYER = {
    "self_attn.q_proj": "attention.q_proj", "self_attn.k_proj": "attention.k_proj", "self_attn.v_proj": "attention.v_proj", "self_attn.output_proj": "attention.out_proj",
    "self_attn_layer_norm": "layer_norm", "ffn.inner_proj": "feed_forward.intermediate_dense", "ffn.output_proj": "feed_forward.output_dense", "ffn_layer_norm": "final_layer_norm",
}


def key_map(n_layers):
    mapping = dict(STATIC)
    for i in range(7):
        for suffix in ("conv.weight", "conv.bias", "layer_norm.weight", "layer_norm.bias"):
            mapping["encoder_frontend.feature_extractor.layers.%d.%s" % (i, suffix)] = "wav2vec2.feature_extractor.conv_layers.%d.%s" % (i, suffix)
    for i in range(n_layers):
        for source, target in LAYER.items():
            for part in ("weight", "bias"):
                mapping["encoder.layers.%d.%s.%s" % (i, source, part)] = "wav2vec2.encoder.layers.%d.%s.%s" % (i, target, part)
    return mapping


def load_state(path):
    import torch
    obj = torch.load(path, map_location="cpu", weights_only=True)
    state = obj.get("model", obj) if isinstance(obj, dict) else obj
    if not isinstance(state, dict) or not all(hasattr(v, "shape") for v in state.values()):
        raise SystemExit("the checkpoint is not a flat dictionary of tensors: keys %s" % list(obj.keys())[:10] if isinstance(obj, dict) else type(obj))
    return state


def architecture(state):
    layers = {int(k.split(".")[2]) for k in state if k.startswith("encoder.layers.")}
    hidden = state["encoder.layer_norm.weight"].shape[0]
    return {"n_layers": len(layers), "vocab_size": state["final_proj.weight"].shape[0], "hidden": hidden, "intermediate": state["encoder.layers.0.ffn.inner_proj.weight"].shape[0], "heads": hidden // 64}


def build(state, blank_id=0):
    import torch
    from transformers import Wav2Vec2Config, Wav2Vec2ForCTC
    a = architecture(state)
    config = Wav2Vec2Config(hidden_size=a["hidden"], num_hidden_layers=a["n_layers"], num_attention_heads=a["heads"], intermediate_size=a["intermediate"], hidden_act="gelu",
                            do_stable_layer_norm=True, feat_extract_norm="layer", conv_dim=(512,) * 7, conv_stride=(5, 2, 2, 2, 2, 2, 2), conv_kernel=(10, 3, 3, 3, 3, 2, 2), conv_bias=True,
                            num_conv_pos_embeddings=128, num_conv_pos_embedding_groups=16, vocab_size=a["vocab_size"], pad_token_id=blank_id, add_adapter=False)
    model = Wav2Vec2ForCTC(config)
    mapping = key_map(a["n_layers"])
    leftovers = sorted(k for k in state if k not in mapping)
    if leftovers:
        raise SystemExit("%d checkpoint tensors have no place in the model: %s" % (len(leftovers), leftovers[:8]))
    renamed = {mapping[k]: v.float() for k, v in state.items()}
    expected = set(model.state_dict())
    if "wav2vec2.masked_spec_embed" in expected:
        renamed["wav2vec2.masked_spec_embed"] = torch.zeros(a["hidden"])      # used only when training with masking
    missing, unexpected = model.load_state_dict(renamed, strict=False)
    if missing or unexpected:
        raise SystemExit("the mapping does not fit: missing %s, unexpected %s" % (missing[:8], unexpected[:8]))
    return model.eval(), a


def convert(pt_path, spm_path, out_dir):
    pieces = sentencepiece_pieces(spm_path)
    state = load_state(pt_path)
    model, a = build(state)
    if len(pieces) != a["vocab_size"]:
        raise SystemExit("the vocabulary has %d pieces but the CTC head has %d outputs" % (len(pieces), a["vocab_size"]))
    os.makedirs(out_dir, exist_ok=True)
    model.save_pretrained(out_dir)
    with open(os.path.join(out_dir, "pieces.json"), "w", encoding="utf-8") as handle:
        json.dump([text for text, _ in pieces], handle, ensure_ascii=False)
    with open(os.path.join(out_dir, "SOURCE.txt"), "w", encoding="utf-8") as handle:
        handle.write("converted from %s (%s) with tools/media_data/omni_ctc.py; architecture %s; %d pieces; blank = piece 0 %r\n" % (os.path.basename(pt_path), os.path.basename(spm_path), a, len(pieces), pieces[0][0]))
    print("converted:", a, "first pieces", [t for t, _ in pieces[:6]])
    return 0


# ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ reading


ARABIC_RANGES = ((0x0600, 0x06FF), (0x0750, 0x077F), (0xFB50, 0xFDFF), (0xFE70, 0xFEFF))


def allowed_pieces(pieces, ranges=ARABIC_RANGES):
    """[True/False] per vocabulary piece: may the decoder emit it when the output must be in the script of `ranges`? The model writes the same spoken Urdu in Devanagari (as Hindi) when it is not told which
    script is wanted, because a CTC model takes no language. Allowed: the special tokens, the word marker and spaces, digits and punctuation, and letters inside the ranges. A piece with a letter of any
    other script is not allowed. (Letters and combining marks decide: a piece of digits or punctuation is script-neutral.)"""
    import unicodedata
    allowed = []
    for piece in pieces:
        if piece.startswith("<") and piece.endswith(">"):
            allowed.append(True)
            continue
        ok = True
        for ch in piece:
            if unicodedata.category(ch)[0] in ("L", "M") and not any(lo <= ord(ch) <= hi for lo, hi in ranges):      # letters AND marks: a Devanagari vowel sign is a mark, not a letter
                ok = False
                break
        allowed.append(ok)
    return allowed


class OmniCTC:
    """The converted model: 16 kHz mono samples in, text out (greedy CTC). Pieces longer than 40 seconds are the caller's to cut (speech_lab.split_for_limit)."""

    def __init__(self, folder, threads=8, script=None):
        import torch
        from transformers import Wav2Vec2ForCTC
        torch.set_num_threads(threads)
        self.torch = torch
        self.model = Wav2Vec2ForCTC.from_pretrained(folder).eval()
        with open(os.path.join(folder, "pieces.json"), encoding="utf-8") as handle:
            self.pieces = json.load(handle)
        self.penalty = None
        if script == "arabic":      # decoding constrained to Arabic-script letters (amendment A8 of docs/data/speech-lab-preregistration-20261112.md)
            self.penalty = torch.tensor([0.0 if ok else -1e4 for ok in allowed_pieces(self.pieces)])

    def frames(self, samples):
        torch = self.torch
        x = torch.as_tensor(samples, dtype=torch.float32)
        x = (x - x.mean()) / (x.std() + 1e-7)       # instance normalisation, as in training
        with torch.inference_mode():
            logits = self.model(x.unsqueeze(0)).logits[0]
        if self.penalty is not None:
            logits = logits + self.penalty
        return logits.argmax(-1).tolist()

    def decode(self, ids, blank=0):
        out, previous = [], -1
        for i in ids:
            if i != blank and i != previous:
                out.append(self.pieces[i])
            previous = i
        text = "".join(p for p in out if not (p.startswith("<") and p.endswith(">")))
        return text.replace("▁", " ").strip()

    def transcribe(self, samples):
        return self.decode(self.frames(samples))


def main(argv):
    if len(argv) >= 2 and argv[0] == "describe":
        state = load_state(argv[1])
        print(architecture(state), "tensors", len(state))
        for k in list(state)[:6] + [k for k in state if "pos_encoder" in k][:4]:
            print(" ", k, tuple(state[k].shape))
        return 0
    if len(argv) == 4 and argv[0] == "convert":
        return convert(argv[1], argv[2], argv[3])
    raise SystemExit(__doc__)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
