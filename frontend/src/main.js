import "./style.css";
import { api, native, onState } from "./bridge";
document.querySelector("#app").innerHTML = `
 <header class="toolbar"><div class="brand"><span class="brand-mark">↔</span><div><h1>Recoil Practice</h1><span class="eyebrow">STRAFE TRAINER / 01</span></div></div><div class="window-actions"><button id="quit" title="Close application">×</button></div></header>
 <main>
  <section class="stage" aria-label="Strafe guidance">
   <div class="alignment"><span></span><small>PREVIEW</small><span></span></div>
   <div class="arrows"><div id="left" class="arrow left" aria-label="Strafe left"><svg viewBox="0 0 64 64" aria-hidden="true"><path d="M37 12 17 32l20 20M18 32h34"/></svg></div><div class="crosshair-guide" aria-hidden="true">+</div><div id="right" class="arrow right" aria-label="Strafe right"><svg viewBox="0 0 64 64" aria-hidden="true"><path d="m27 12 20 20-20 20M46 32H12"/></svg></div></div>
   <div id="timeline" class="timeline"><div class="timeline-top"><span>R-301 <small>EXPECTED STRAFE</small></span><span id="elapsed">0.00 / 2.21 s</span></div><div class="track"><div class="segment right-segment" style="flex:800"><span>R <small>800 ms</small></span></div><div class="segment left-segment" style="flex:530"><span>L <small>530 ms</small></span></div><div class="segment right-segment" style="flex:880"><span>R <small>880 ms</small></span></div><div id="playhead"></div></div><div class="ticks"><span>0</span><span>0.80</span><span>1.33</span><span>2.21 s</span></div></div>
  </section>
  <section class="controls">
   <div class="section-heading"><span>OVERLAY SETUP</span><span class="drag-hint">Drag the header · resize the window</span></div>
   <div class="settings-grid">
    <label>Arrow spacing <output id="gap-value"></output><input id="gap" type="range" min="40" max="300" step="2"></label>
    <label>Arrow size <output id="arrowSize-value"></output><input id="arrowSize" type="range" min="24" max="72" step="2"></label>
    <label>Opacity <output id="opacity-value"></output><input id="opacity" type="range" min="20" max="100"></label>
    <label>Timeline spacing <output id="timelineOffset-value"></output><input id="timelineOffset" type="range" min="12" max="100" step="2"></label>
   </div>
   <div class="options-row"><label class="check"><input id="arrows-toggle" type="checkbox">Arrows</label><label class="check"><input id="timeline-toggle" type="checkbox">Timeline</label><label class="check" title="Hide the timeline in practice while left-click is held"><input id="timelineIdle-toggle" type="checkbox">Hide timeline while shooting</label></div><div class="options-row second"><label class="check"><input id="voice-toggle" type="checkbox">Voice</label><label class="lead-label">Voice lead <input id="voiceLeadMs" type="number" min="0" max="350" step="10"><span>ms</span></label><span class="position-actions"><button id="move" class="mini" title="Show the overlay and drag it into place">Move overlay</button><button id="centerOverlay" class="mini" title="Put the overlay back on the crosshair">Center</button></span></div>
   <div class="actions"><button id="preview" class="secondary">▷ Preview pattern</button><button id="lock" class="primary">Start practice <kbd>F8</kbd></button></div>
   <p class="helper">Hold left-click in Apex to begin. Release to reset.<br><kbd>F8</kbd> edit / practice <span class="divider">·</span> <kbd>F9</kbd> disable / enable · then click again</p>
   <p id="error" role="alert" hidden></p>
  </section>
 </main>
 <footer><span id="status-dot" class="status-dot"></span><span id="status">Connecting…</span><span class="version">R-301 · v0.1</span></footer>
 <div id="practice-status" class="practice-status"></div>
`;
const $ = (id) => document.getElementById(id);
let state,
  localSettings,
  pending = false,
  saving = false,
  debounce,
  savePromise;
function error(err) {
  $("error").hidden = false;
  $("error").textContent = String(err);
}
function render(s) {
  state = s;
  if (!pending && !saving) localSettings = { ...s.settings };
  const config = localSettings || s.settings;
  document.body.classList.toggle("practice", !s.editing);
  document.documentElement.style.setProperty("--gap", `${config.gap}px`);
  document.documentElement.style.setProperty(
    "--arrow-size",
    `${config.arrowSize}px`,
  );
  document.documentElement.style.setProperty(
    "--overlay-opacity",
    config.opacity / 100,
  );
  document.documentElement.style.setProperty(
    "--timeline-offset",
    `${config.timelineOffset}px`,
  );
  const controlsTop = Math.max(
    275,
    76 + 66 + config.arrowSize / 2 + config.timelineOffset + 100,
  );
  document.documentElement.style.setProperty(
    "--controls-top",
    `${controlsTop}px`,
  );
  document.documentElement.style.setProperty(
    "--content-height",
    `${controlsTop + 300}px`,
  );
  for (const id of ["gap", "arrowSize", "opacity", "timelineOffset"]) {
    if (document.activeElement !== $(id)) $(id).value = config[id];
    $(`${id}-value`).textContent =
      `${config[id]}${id === "opacity" ? "%" : " px"}`;
  }
  if (document.activeElement !== $("voiceLeadMs"))
    $("voiceLeadMs").value = config.voiceLeadMs;
  $("arrows-toggle").checked = config.arrows;
  $("left").parentElement.style.visibility = config.arrows
    ? "visible"
    : "hidden";
  $("timeline-toggle").checked = config.timeline;
  $("voice-toggle").checked = config.voice;
  $("voiceLeadMs").disabled = !config.voice;
  $("timelineIdle-toggle").checked = config.timelineIdle;
  $("timelineIdle-toggle").disabled = !config.timeline;
  $("timeline").hidden = !config.timeline;
  $("left").classList.toggle(
    "active",
    s.direction === "left" && (s.armed || s.preview),
  );
  $("right").classList.toggle(
    "active",
    s.direction === "right" && (s.armed || s.preview),
  );
  $("elapsed").textContent = `${(s.elapsedMs / 1000).toFixed(2)} / 2.21 s`;
  $("playhead").style.left = `${Math.min(100, (s.elapsedMs / 2210) * 100)}%`;
  $("preview").textContent =
    s.running && s.preview ? "■ Stop preview" : "▷ Preview pattern";
  $("move").textContent = s.moving ? "Done moving" : "Move overlay";
  $("move").classList.toggle("active", s.moving);
  $("lock").disabled = !s.inputReady || saving || pending;
  let status = !native
    ? "Browser preview · global input unavailable"
    : !s.inputReady
      ? "Input unavailable"
      : !s.armed
        ? "Paused · F9 to resume"
        : s.moving
          ? "Drag the overlay onto your crosshair"
          : s.editing
            ? "Edit mode · adjust your overlay"
            : !s.focused
              ? "Waiting for Apex Legends"
              : s.running
                ? "Follow the highlighted arrow"
                : s.held
                  ? "Pattern complete · release to reset"
                  : "Ready · hold left-click";
  $("status").textContent = status;
  $("status-dot").classList.toggle("live", s.inputReady && s.armed);
  $("practice-status").textContent = !s.armed
    ? "DISABLED · F9 TO ENABLE"
    : !s.focused
      ? "WAITING FOR APEX · F8 TO EDIT"
      : s.held && !s.running
        ? "RELEASE TO RESET"
        : "F8 EDIT · F9 DISABLE";
  $("error").hidden = !s.error;
  if (s.error) $("error").textContent = s.error;
}
async function flushSettings() {
  clearTimeout(debounce);
  if (savePromise) {
    await savePromise;
    return flushSettings();
  }
  if (!pending) return;
  pending = false;
  saving = true;
  savePromise = api.UpdateSettings({ ...localSettings });
  try {
    const result = await savePromise;
    saving = false;
    render(result);
  } catch (e) {
    saving = false;
    error(e);
    throw e;
  } finally {
    savePromise = null;
  }
  if (pending) await flushSettings();
}
function change(key, value) {
  localSettings = { ...localSettings, [key]: value };
  pending = true;
  render(state);
  clearTimeout(debounce);
  debounce = setTimeout(() => flushSettings().catch(error), 180);
}
for (const key of ["gap", "arrowSize", "opacity", "timelineOffset"])
  $(key).addEventListener("input", (e) => change(key, Number(e.target.value)));
$("voiceLeadMs").addEventListener("change", (e) =>
  change(
    "voiceLeadMs",
    Math.max(0, Math.min(350, Number(e.target.value) || 0)),
  ),
);
$("arrows-toggle").addEventListener("change", (e) =>
  change("arrows", e.target.checked),
);
$("timeline-toggle").addEventListener("change", (e) =>
  change("timeline", e.target.checked),
);
$("timelineIdle-toggle").addEventListener("change", (e) =>
  change("timelineIdle", e.target.checked),
);
$("voice-toggle").addEventListener("change", (e) =>
  change("voice", e.target.checked),
);
$("preview").onclick = async () => {
  try {
    await flushSettings();
    if (state.running && state.preview) await api.StopPreview();
    else render(await api.Preview());
  } catch (e) {
    error(e);
  }
};
$("lock").onclick = async () => {
  try {
    await flushSettings();
    render(await api.ToggleMode());
  } catch (e) {
    error(e);
  }
};
$("move").onclick = async () => {
  try {
    await flushSettings();
    render(await api.ToggleMove());
  } catch (e) {
    error(e);
  }
};
$("centerOverlay").onclick = () =>
  api.CenterOverlay().then(render).catch(error);
$("quit").onclick = () => api.Quit().catch(error);
document.addEventListener("keydown", (e) => {
  if (e.code === "Escape" && state?.preview) api.StopPreview().catch(error);
});
onState(render);
api.GetState().then(render).catch(error);
