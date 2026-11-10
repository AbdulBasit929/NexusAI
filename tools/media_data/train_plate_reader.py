#!/usr/bin/env python3
"""Train the plate reader (docs/data/plates-m5-preregistration-20261110.md): a small CRNN with CTC loss, from scratch, on synthetic Pakistani-format plates, exported to ONNX.

    docker run --rm --network none --cpus 6 -m 2g -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro nexusai-media-eval:latest \
        python /repo/tools/media_data/train_plate_reader.py --out /data/models/plates/reader-v1 --epochs 14 --samples 30000

Crops are generated fresh every epoch (plates_synth.render_plate, then degrade); the validation set is a fixed 1,500 crops from another seed. Reported: greedy exact and character error, and the
exact rate after the registry-constrained choice (plate_formats.choose over a CTC beam), which is what the product uses. No pretrained weights: the model is ours by construction.
"""
import argparse
import hashlib
import json
import os
import random
import sys
import time
from pathlib import Path

import numpy as np
import torch
import torch.nn as nn
from torch.utils.data import DataLoader, IterableDataset, get_worker_info

sys.path.insert(0, os.environ.get("NEXUSAI_INGEST", "/repo/ingestion/forensic_records"))
sys.path.insert(0, str(Path(__file__).resolve().parent))
import cv2  # noqa: E402
import plate_formats as pf  # noqa: E402
import plate_reader as pr  # noqa: E402
import plates_synth as ps  # noqa: E402

CLASSES = len(pr.CHARSET) + 1
STEPS = pr.WIDTH // 4  # time steps after the network's horizontal /4


def sample(rng):
    plate = pf.random_plate(rng)
    crop = ps.degrade(rng, ps.render_plate(rng, plate))
    return pr.prepare_crop(crop, cv2), plate["norm"], plate


class Stream(IterableDataset):
    def __init__(self, count, seed):
        self.count, self.seed = count, seed

    def __iter__(self):
        info = get_worker_info()
        workers, index = (info.num_workers, info.id) if info else (1, 0)
        rng = random.Random(self.seed * 1000003 + index)
        for _ in range(self.count // workers):
            image, text, _ = sample(rng)
            yield torch.from_numpy(image), torch.tensor([pr.CHARSET.index(c) + 1 for c in text], dtype=torch.long)


def collate(batch):
    images = torch.stack([b[0] for b in batch])
    targets = torch.cat([b[1] for b in batch])
    lengths = torch.tensor([len(b[1]) for b in batch], dtype=torch.long)
    return images, targets, lengths


class Reader(nn.Module):
    def __init__(self):
        super().__init__()

        def block(i, o, pool):
            layers = [nn.Conv2d(i, o, 3, padding=1, bias=False), nn.BatchNorm2d(o), nn.ReLU(inplace=True)]
            if pool:
                layers.append(nn.MaxPool2d(pool))
            return layers
        self.features = nn.Sequential(*block(1, 32, (2, 2)), *block(32, 64, (2, 2)), *block(64, 128, None), *block(128, 128, (2, 1)), *block(128, 192, (2, 1)))
        self.rnn = nn.GRU(192 * 2, 128, num_layers=2, bidirectional=True, batch_first=True, dropout=0.1)
        self.head = nn.Linear(256, CLASSES)

    def forward(self, x):  # x: (B, 1, 32, 192) -> logits (B, T, C)
        f = self.features(x)  # (B, 192, 2, 48)
        b, c, h, w = f.shape
        f = f.permute(0, 3, 1, 2).reshape(b, w, c * h)
        out, _ = self.rnn(f)
        return self.head(out)


def greedy(logits):
    best = logits.argmax(-1).cpu().numpy()
    texts = []
    for row in best:
        chars, previous = [], 0
        for cls in row:
            if cls != previous and cls != 0:
                chars.append(pr.CHARSET[cls - 1])
            previous = cls
        texts.append("".join(chars))
    return texts


def edit_distance(a, b):
    row = list(range(len(b) + 1))
    for i in range(1, len(a) + 1):
        prev, row[0] = row[0], i
        for j in range(1, len(b) + 1):
            prev, row[j] = row[j], min(row[j] + 1, row[j - 1] + 1, prev + (a[i - 1] != b[j - 1]))
    return row[len(b)]


def validation_set(count, seed):
    rng = random.Random(seed)
    items = [sample(rng) for _ in range(count)]
    return torch.from_numpy(np.stack([i[0] for i in items])), [i[1] for i in items]


@torch.no_grad()
def evaluate(model, images, truths, constrained=False):
    model.eval()
    logits = torch.cat([model(images[i:i + 256]) for i in range(0, len(images), 256)])
    reads = greedy(logits)
    exact = sum(r == t for r, t in zip(reads, truths)) / len(truths)
    cer = sum(edit_distance(r, t) for r, t in zip(reads, truths)) / sum(len(t) for t in truths)
    result = {"exact": round(exact, 4), "cer": round(cer, 4)}
    if constrained:
        log_probs = torch.log_softmax(logits, -1).cpu().numpy()
        chosen = 0
        for lp, truth in zip(log_probs, truths):
            text, _, _ = pf.choose(pr.ctc_beam(lp))
            chosen += text == truth
        result["exact_after_format_choice"] = round(chosen / len(truths), 4)
    model.train()
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--out", required=True)
    parser.add_argument("--epochs", type=int, default=14)
    parser.add_argument("--samples", type=int, default=30000, help="crops generated per epoch")
    parser.add_argument("--batch", type=int, default=96)
    parser.add_argument("--workers", type=int, default=4)
    parser.add_argument("--val", type=int, default=1500)
    parser.add_argument("--seed", type=int, default=20261110)
    parser.add_argument("--lr", type=float, default=2e-3)
    parser.add_argument("--export-only", action="store_true", help="load reader.pt from --out and write the ONNX file")
    args = parser.parse_args()
    if args.export_only:
        model = Reader()
        model.load_state_dict(torch.load(Path(args.out) / "reader.pt", map_location="cpu"))
        export_onnx(model, Path(args.out))
        return
    torch.manual_seed(args.seed)
    torch.set_num_threads(max(1, os.cpu_count() // 2))
    out = Path(args.out)
    out.mkdir(parents=True, exist_ok=True)
    val_images, val_truths = validation_set(args.val, 999)
    model = Reader()
    parameters = sum(p.numel() for p in model.parameters())
    print(f"reader parameters {parameters:,}; fonts {len(ps.FONTS)}; validation crops {len(val_truths)}", flush=True)
    optimiser = torch.optim.AdamW(model.parameters(), lr=args.lr, weight_decay=1e-4)
    steps_per_epoch = args.samples // args.batch
    schedule = torch.optim.lr_scheduler.OneCycleLR(optimiser, max_lr=args.lr, total_steps=args.epochs * steps_per_epoch + 5)
    ctc = nn.CTCLoss(blank=0, zero_infinity=True)
    best = (-1.0, None)
    history = []
    started = time.time()
    for epoch in range(1, args.epochs + 1):
        loader = DataLoader(Stream(args.samples, args.seed + epoch), batch_size=args.batch, num_workers=args.workers, collate_fn=collate, drop_last=True)
        total, count = 0.0, 0
        for images, targets, lengths in loader:
            logits = model(images)
            log_probs = torch.log_softmax(logits, -1).permute(1, 0, 2)
            loss = ctc(log_probs, targets, torch.full((images.shape[0],), STEPS, dtype=torch.long), lengths)
            optimiser.zero_grad()
            loss.backward()
            nn.utils.clip_grad_norm_(model.parameters(), 5.0)
            optimiser.step()
            schedule.step()
            total, count = total + float(loss), count + 1
        metrics = evaluate(model, val_images, val_truths)
        history.append({"epoch": epoch, "loss": round(total / max(1, count), 4), **metrics, "seconds": round(time.time() - started)})
        print(json.dumps(history[-1]), flush=True)
        if metrics["exact"] > best[0]:
            best = (metrics["exact"], {k: v.clone() for k, v in model.state_dict().items()})
            torch.save(best[1], out / "reader.pt")
    model.load_state_dict(best[1])
    final = evaluate(model, val_images, val_truths, constrained=True)
    print("best", json.dumps(final), flush=True)
    meta = {"model": "nexus-plate-reader-crnn-ctc", "version": 1, "registry_version": pf.load_registry()["registry_version"], "charset": pr.CHARSET, "input": [1, pr.HEIGHT, pr.WIDTH],
            "parameters": parameters, "seed": args.seed, "epochs": args.epochs, "samples_per_epoch": args.samples, "validation": final, "history": history,
            "trained": time.strftime("%Y-%m-%dT%H:%M:%S"), "pretrained_weights": None, "data": "synthetic plates from plate_format_registry_v1 (tier T1)"}
    (out / "reader-meta.json").write_text(json.dumps(meta, indent=1), encoding="utf-8")
    export_onnx(model, out)


def export_onnx(model, out):
    """Write plate-reader-v1.onnx beside the checkpoint. A missing exporter dependency is reported and the checkpoint stays valid: run again with --export-only."""
    model.eval()
    onnx_path = out / "plate-reader-v1.onnx"
    try:
        torch.onnx.export(model, torch.zeros(1, 1, pr.HEIGHT, pr.WIDTH), str(onnx_path), input_names=["crop"], output_names=["logits"],
                          dynamic_axes={"crop": {0: "batch"}, "logits": {0: "batch"}}, opset_version=17, dynamo=False)
    except Exception as error:  # noqa: BLE001
        print(f"ONNX export failed ({error.__class__.__name__}: {error}); the checkpoint {out / 'reader.pt'} is saved, rerun with --export-only", flush=True)
        return
    sha = hashlib.sha256(onnx_path.read_bytes()).hexdigest()
    meta_path = out / "reader-meta.json"
    meta = json.loads(meta_path.read_text(encoding="utf-8")) if meta_path.exists() else {}
    meta["onnx_sha256"] = sha
    meta_path.write_text(json.dumps(meta, indent=1), encoding="utf-8")
    print(f"exported {onnx_path} sha256 {sha[:16]}", flush=True)


if __name__ == "__main__":
    main()
