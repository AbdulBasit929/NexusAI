const query = new URLSearchParams(location.search);
const token = query.get("token") || "";
let state;
let imageIndex = 0;
let speechIndex = 0;
let startSeconds = 0;
let endSeconds = 0;

const byId = (id) => document.getElementById(id);

function toast(message) {
  const node = byId("toast");
  node.textContent = message;
  node.classList.add("show");
  setTimeout(() => node.classList.remove("show"), 3600);
}

async function loadState() {
  const response = await fetch("/api/state", {cache: "no-store"});
  state = await response.json();
  render();
}

async function action(payload) {
  const response = await fetch("/api/action", {
    method: "POST",
    headers: {"Content-Type": "application/json", "X-NXMMR-Token": token},
    body: JSON.stringify(payload),
  });
  const result = await response.json();
  if (!response.ok) throw new Error(result.error || "Request failed");
  state = result;
  render();
}

function currentImageList() {
  return state.development_frozen ? state.images.sealed_holdout : state.images.development;
}

function render() {
  byId("reviewer").value = state.reviewer || "";
  byId("statusPill").textContent = state.locked ? "LOCKED · PASS" : state.validation.status.replaceAll("_", " ");
  byId("progress").textContent = `Images ${state.progress.development}/${state.progress.development_total} dev · ${state.progress.holdout}/${state.progress.holdout_total} holdout · Speech ${state.progress.speech}/${state.progress.speech_total}`;
  renderImage();
  renderVideo();
  renderSpeech();
  renderValidation();
  document.querySelectorAll("button, input, select, textarea").forEach((node) => {
    if (state.locked && node.id !== "previousImage" && node.id !== "nextImage" && node.id !== "previousSpeech" && node.id !== "nextSpeech") node.disabled = true;
  });
}

function renderImage() {
  const list = currentImageList();
  if (imageIndex >= list.length) imageIndex = Math.max(0, list.length - 1);
  const item = list[imageIndex];
  const stage = state.development_frozen ? "SEALED HOLDOUT · DO NOT TUNE" : "DEVELOPMENT SANITY SET";
  byId("imageStage").textContent = stage;
  byId("imageTitle").textContent = `${item.sample_id} · ${imageIndex + 1} of ${list.length}`;
  byId("reviewImage").src = `${item.media_url}?v=${item.sha256.slice(0, 10)}`;
  byId("imageMeta").textContent = `${item.relative_file} · ${item.width}×${item.height} · SHA-256 ${item.sha256}`;
  const gated = item.independence !== "unseen";
  byId("independenceGate").hidden = !gated;
  byId("imageForm").hidden = gated;
  const label = item.label || {};
  byId("platePresence").value = label.plate_presence || "";
  byId("plateCount").value = label.plate_count ?? 0;
  byId("plateText").value = label.plate_text || "";
  byId("readability").value = label.readability || "";
  byId("difficulty").value = label.difficulty || "";
  byId("imageNotes").value = label.review_notes || "";
  byId("previousImage").disabled = imageIndex === 0;
  byId("nextImage").disabled = imageIndex >= list.length - 1;
  const devComplete = state.progress.development === state.progress.development_total;
  byId("freezeDevelopment").hidden = state.development_frozen || !devComplete;
}

function renderVideo() {
  if (!byId("reviewVideo").src) byId("reviewVideo").src = state.video.url;
  byId("watchedFullVideo").checked = state.video.review.watched_full_source;
  byId("videoNotes").value = state.video.review.review_notes || "";
  const records = state.video.events.map((event) => `
    <div class="record"><div><strong>${event.event_type.toUpperCase()}</strong> · ${event.start_seconds.toFixed(3)}–${event.end_seconds.toFixed(3)}s · ${event.plate_count} plate(s) ${escapeHtml(event.plate_text || "")}</div>
    <button class="danger delete-event" data-event="${event.event_id}">Delete</button></div>`).join("");
  byId("videoEvents").innerHTML = records || '<p class="meta">No intervals saved yet.</p>';
  document.querySelectorAll(".delete-event").forEach((button) => button.addEventListener("click", () => run(() => action({action: "delete_video_event", event_id: button.dataset.event}))));
}

function renderSpeech() {
  if (!byId("reviewAudio").src) byId("reviewAudio").src = state.speech.audio_url;
  byId("findSpeech").hidden = state.speech.segments.length > 0;
  byId("vadStatus").textContent = state.speech.vad_receipt ? `${state.speech.segments.length} bounded candidates from audio energy; no semantic model used.` : "No candidates generated yet.";
  if (!state.speech.segments.length) {
    byId("speechCard").innerHTML = "";
    return;
  }
  if (speechIndex >= state.speech.segments.length) speechIndex = state.speech.segments.length - 1;
  const item = state.speech.segments[speechIndex];
  byId("speechCard").innerHTML = `
    <form id="speechForm" class="speech-panel">
      <h3>${item.segment_id} · ${speechIndex + 1} of ${state.speech.segments.length} · ${item.start_seconds.toFixed(3)}–${item.end_seconds.toFixed(3)}s</h3>
      <button type="button" id="playSpeech" class="secondary">Play this segment</button>
      <label>Speech present? <select id="speechPresence"><option value="">Choose…</option><option value="yes">Yes</option><option value="no">No</option></select></label>
      <label>Raw verbatim transcript <span class="hint">Urdu script for spoken Urdu; do not romanize</span><textarea id="rawTranscript" rows="4" dir="auto"></textarea></label>
      <label>Language <select id="speechLanguage"><option value="">Choose…</option><option value="urdu">Urdu</option><option value="english">English</option><option value="mixed">Mixed</option><option value="other">Other</option><option value="unknown">Unknown</option><option value="not_applicable">Not applicable</option></select></label>
      <label><input id="unintelligible" type="checkbox"> Speech is unintelligible</label>
      <label>Optional notes <textarea id="speechNotes" rows="2"></textarea></label>
      <button type="submit">Save &amp; next segment</button>
    </form>`;
  byId("speechPresence").value = item.speech_present || "";
  byId("rawTranscript").value = item.transcript_raw || "";
  byId("speechLanguage").value = item.transcript_language || "";
  byId("unintelligible").checked = Boolean(item.unintelligible);
  byId("speechNotes").value = item.review_notes || "";
  byId("playSpeech").addEventListener("click", () => playBounded(item.start_seconds, item.end_seconds));
  byId("speechForm").addEventListener("submit", (event) => {
    event.preventDefault();
    run(async () => {
      await action({action: "save_speech", segment_id: item.segment_id, speech_present: byId("speechPresence").value, transcript_raw: byId("rawTranscript").value, transcript_language: byId("speechLanguage").value, unintelligible: byId("unintelligible").checked, review_notes: byId("speechNotes").value});
      speechIndex = Math.min(speechIndex + 1, state.speech.segments.length - 1);
      renderSpeech();
    });
  });
  byId("previousSpeech").disabled = speechIndex === 0;
  byId("nextSpeech").disabled = speechIndex >= state.speech.segments.length - 1;
}

function renderValidation() {
  const validation = state.validation;
  if (validation.status === "PASS") {
    byId("validation").innerHTML = `<div class="validation-pass"><strong>GroundTruthValidation=PASS</strong><br>Gold labels are locked. Model comparison remains separate.</div>`;
    byId("finishReview").hidden = true;
  } else if (validation.status === "READY_TO_LOCK") {
    byId("validation").innerHTML = '<div class="validation-pass"><strong>Ready to lock.</strong> All required human fields and attestations validate.</div>';
    byId("finishReview").hidden = false;
  } else {
    const shown = validation.errors.slice(0, 12).map((error) => `<li>${escapeHtml(error)}</li>`).join("");
    const more = validation.errors.length > 12 ? `<li>…and ${validation.errors.length - 12} more.</li>` : "";
    byId("validation").innerHTML = `<div class="validation-errors"><strong>GroundTruthValidation=PENDING</strong><ul>${shown}${more}</ul></div>`;
    byId("finishReview").hidden = false;
  }
}

function escapeHtml(value) {
  return String(value).replace(/[&<>'"]/g, (character) => ({"&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", '"': "&quot;"}[character]));
}

async function playBounded(start, end) {
  const audio = byId("reviewAudio");
  audio.currentTime = start;
  await audio.play();
  const stop = () => {
    if (audio.currentTime >= end || audio.paused) {
      audio.pause();
      audio.removeEventListener("timeupdate", stop);
    }
  };
  audio.addEventListener("timeupdate", stop);
}

async function proposeSpeechSegments() {
  byId("vadStatus").textContent = "Decoding local review audio and measuring signal energy…";
  const bytes = await fetch(state.speech.audio_url).then((response) => response.arrayBuffer());
  const context = new AudioContext();
  const buffer = await context.decodeAudioData(bytes);
  const windowSeconds = 0.1;
  const windowSamples = Math.max(1, Math.floor(buffer.sampleRate * windowSeconds));
  const energies = [];
  for (let offset = 0; offset < buffer.length; offset += windowSamples) {
    const limit = Math.min(buffer.length, offset + windowSamples);
    let squares = 0;
    let samples = 0;
    for (let channel = 0; channel < buffer.numberOfChannels; channel += 1) {
      const data = buffer.getChannelData(channel);
      for (let index = offset; index < limit; index += 4) {
        squares += data[index] * data[index];
        samples += 1;
      }
    }
    energies.push(20 * Math.log10(Math.sqrt(squares / Math.max(1, samples)) + 1e-8));
  }
  const sorted = [...energies].sort((a, b) => a - b);
  const noiseFloor = sorted[Math.floor(sorted.length * 0.25)];
  const peakFloor = sorted[Math.floor(sorted.length * 0.75)];
  const thresholdDb = Math.max(noiseFloor + 4, Math.min(peakFloor, noiseFloor + 9));
  const raw = [];
  let activeStart = null;
  energies.forEach((energy, index) => {
    if (energy >= thresholdDb && activeStart === null) activeStart = index * windowSeconds;
    if ((energy < thresholdDb || index === energies.length - 1) && activeStart !== null) {
      const end = Math.min(buffer.duration, (index + 1) * windowSeconds);
      if (end - activeStart >= 0.3) raw.push({start_seconds: Math.max(0, activeStart - 0.2), end_seconds: Math.min(buffer.duration, end + 0.25)});
      activeStart = null;
    }
  });
  const merged = [];
  raw.forEach((segment) => {
    const prior = merged.at(-1);
    if (prior && segment.start_seconds - prior.end_seconds <= 0.45 && segment.end_seconds - prior.start_seconds <= 15) prior.end_seconds = segment.end_seconds;
    else merged.push({...segment});
  });
  let bounded = [];
  merged.forEach((segment) => {
    let cursor = segment.start_seconds;
    while (segment.end_seconds - cursor > 15) {
      bounded.push({start_seconds: cursor, end_seconds: cursor + 12});
      cursor += 12;
    }
    if (segment.end_seconds - cursor >= 0.3) bounded.push({start_seconds: cursor, end_seconds: segment.end_seconds});
  });
  let fallback = null;
  if (!bounded.length) {
    fallback = sorted.at(-1) <= -150 ? "digital_silence_audit" : "no_active_window_audit";
    const auditWindows = 6;
    bounded = Array.from({length: auditWindows}, (_, index) => ({
      start_seconds: buffer.duration * index / auditWindows,
      end_seconds: buffer.duration * (index + 1) / auditWindows,
    }));
  }
  await context.close();
  if (bounded.length > 30) throw new Error(`Energy produced ${bounded.length} candidates; this exceeds the bounded-review limit and needs deterministic parameter refinement.`);
  await action({
    action: "set_speech_segments",
    segments: bounded.map((item) => ({start_seconds: Number(item.start_seconds.toFixed(3)), end_seconds: Number(item.end_seconds.toFixed(3))})),
    receipt: {method: "browser_audio_energy_v1", window_seconds: windowSeconds, noise_percentile: 0.25, speech_percentile: 0.75, threshold_db: Number(thresholdDb.toFixed(3)), peak_db: Number(sorted.at(-1).toFixed(3)), fallback, merge_gap_seconds: 0.45, padding_seconds: [0.2, 0.25], max_segment_seconds: 15, sample_rate: buffer.sampleRate, audio_duration_seconds: Number(buffer.duration.toFixed(3))},
  });
}

async function run(work) {
  try { await work(); } catch (error) { toast(error.message); }
}

document.querySelectorAll("nav button").forEach((button) => button.addEventListener("click", () => {
  document.querySelectorAll("nav button").forEach((item) => item.classList.toggle("active", item === button));
  document.querySelectorAll(".view").forEach((view) => view.classList.toggle("active", view.id === button.dataset.view));
}));

byId("saveReviewer").addEventListener("click", () => run(() => action({action: "set_reviewer", reviewer: byId("reviewer").value})));
byId("previousImage").addEventListener("click", () => { imageIndex = Math.max(0, imageIndex - 1); renderImage(); });
byId("nextImage").addEventListener("click", () => { imageIndex = Math.min(currentImageList().length - 1, imageIndex + 1); renderImage(); });
byId("markUnseen").addEventListener("click", () => run(() => action({action: "mark_independence", sample_id: currentImageList()[imageIndex].sample_id, model_output_seen: false})));
byId("markSeen").addEventListener("click", () => run(async () => { const id = currentImageList()[imageIndex].sample_id; await action({action: "mark_independence", sample_id: id, model_output_seen: true}); toast(`${id} excluded and deterministically replaced.`); }));
byId("platePresence").addEventListener("change", () => {
  if (byId("platePresence").value === "no") {
    byId("plateCount").value = 0; byId("plateText").value = ""; byId("readability").value = "not_applicable";
  } else if (Number(byId("plateCount").value) === 0) byId("plateCount").value = 1;
});
byId("imageForm").addEventListener("submit", (event) => {
  event.preventDefault();
  run(async () => {
    const item = currentImageList()[imageIndex];
    await action({action: "save_image", sample_id: item.sample_id, plate_presence: byId("platePresence").value, plate_count: Number(byId("plateCount").value), plate_text: byId("plateText").value, readability: byId("readability").value, difficulty: byId("difficulty").value, review_notes: byId("imageNotes").value});
    imageIndex = Math.min(imageIndex + 1, currentImageList().length - 1); renderImage();
  });
});
byId("freezeDevelopment").addEventListener("click", () => run(async () => { await action({action: "freeze_development"}); imageIndex = 0; renderImage(); toast("Development labels frozen. Holdout review opened."); }));

byId("reviewVideo").addEventListener("timeupdate", () => { byId("videoCurrent").textContent = byId("reviewVideo").currentTime.toFixed(3); });
byId("previousFrame").addEventListener("click", () => { byId("reviewVideo").currentTime = Math.max(0, byId("reviewVideo").currentTime - 1 / 60); });
byId("nextFrame").addEventListener("click", () => { byId("reviewVideo").currentTime = Math.min(60.010, byId("reviewVideo").currentTime + 1 / 60); });
byId("setStart").addEventListener("click", () => { startSeconds = byId("reviewVideo").currentTime; byId("startValue").textContent = startSeconds.toFixed(3); });
byId("setEnd").addEventListener("click", () => { endSeconds = byId("reviewVideo").currentTime; byId("endValue").textContent = endSeconds.toFixed(3); });
byId("eventType").addEventListener("change", () => {
  const negative = byId("eventType").value === "negative";
  byId("eventPlateCount").disabled = negative; byId("eventPlateText").disabled = negative; byId("eventReadability").disabled = negative;
});
byId("videoForm").addEventListener("submit", (event) => {
  event.preventDefault();
  run(() => action({action: "save_video_event", start_seconds: startSeconds, end_seconds: endSeconds, event_type: byId("eventType").value, plate_count: Number(byId("eventPlateCount").value), plate_text: byId("eventPlateText").value, readability: byId("eventReadability").value, review_notes: byId("eventNotes").value}));
});
byId("saveVideoReview").addEventListener("click", () => run(() => action({action: "set_video_review", watched_full_source: byId("watchedFullVideo").checked, review_notes: byId("videoNotes").value})));

byId("findSpeech").addEventListener("click", () => run(proposeSpeechSegments));
byId("previousSpeech").addEventListener("click", () => { speechIndex = Math.max(0, speechIndex - 1); renderSpeech(); });
byId("nextSpeech").addEventListener("click", () => { speechIndex = Math.min(state.speech.segments.length - 1, speechIndex + 1); renderSpeech(); });
byId("finishReview").addEventListener("click", () => run(async () => { await action({action: "finish"}); toast("GroundTruthValidation=PASS. Gold labels locked."); }));

document.addEventListener("keydown", (event) => {
  const editing = ["INPUT", "SELECT", "TEXTAREA"].includes(event.target.tagName);
  if (!editing && document.querySelector("#images.active") && event.key === "ArrowLeft") byId("previousImage").click();
  if (!editing && document.querySelector("#images.active") && event.key === "ArrowRight") byId("nextImage").click();
  if ((event.ctrlKey || event.metaKey) && event.key === "Enter") {
    const activeForm = document.querySelector(".view.active form:not([hidden])");
    if (activeForm) { event.preventDefault(); activeForm.requestSubmit(); }
  }
});

loadState().catch((error) => toast(error.message));
