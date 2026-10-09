import "./style.css";
import { api, minimise, native, onState } from "./bridge";
// Weapon icons are optional: a weapon without a file shows its name alone.
const icons = Object.fromEntries(
  Object.entries(
    import.meta.glob("./assets/weapons/*.svg", {
      eager: true,
      query: "?url",
      import: "default",
    }),
  ).map(([path, url]) => [path.match(/([^/]+)\.svg$/)[1], url]),
);
const glyphs = {
  arrows: "M9 7 4 12l5 5M4 12h16M15 7l5 5-5 5",
  timeline: "M3 7h8v4H3zM13 7h8v4h-8zM3 15h12v3H3zM17 4v17",
  overlay: "M3 5h18v12H3zM12 8v6M9 11h6M8 21h8",
  voice: "M4 10v4h3l5 4V6l-5 4zM16 9.5a3.5 3.5 0 0 1 0 5M18.5 7a7 7 0 0 1 0 10",
  keys: "M3 7h18v10H3zM7 10.5h.01M11 10.5h.01M15 10.5h.01M8 13.5h8",
  minimise: "M6 12h12",
  close: "M6 6l12 12M18 6 6 18",
  chevron: "m6 9 6 6 6-6",
};
const icon = (name) =>
  `<svg class="glyph" viewBox="0 0 24 24" aria-hidden="true"><path d="${glyphs[name]}"/></svg>`;
// A slider and a number field for the same setting.
const sliders = {
  gap: ["Spacing", 40, 300, 2, "px"],
  arrowSize: ["Size", 24, 72, 2, "px"],
  timelineOffset: ["Spacing", 12, 400, 2, "px"],
  timelineWidth: ["Width", 280, 800, 4, "px"],
  opacity: ["Opacity", 20, 100, 1, "%"],
};
const slider = (id) => {
  const [label, min, max, step, unit] = sliders[id];
  return `<div class="slider"><span>${label}</span><input id="${id}" type="range" min="${min}" max="${max}" step="${step}" aria-label="${label}"><span class="number"><input id="${id}-number" type="number" min="${min}" max="${max}" step="${step}" aria-label="${label} value"><i>${unit}</i></span></div>`;
};
const card = (name, title, toggle, body) =>
  `<section class="card ${name}"><h2>${icon(name)}<span>${title}</span>${toggle ? `<input id="${toggle}" type="checkbox" class="switch" aria-label="${title}">` : ""}</h2><div class="card-body">${body}</div></section>`;
// Colour schemes; the overlay's colours for each are in render_windows.go.
const themes = {
  mint: "Mint & coral",
  violet: "Violet & gold",
  ember: "Ember & cyan",
  ocean: "Blue & orange",
};
const bindings = {
  left: "Strafe left",
  right: "Strafe right",
  start: "Start practice",
  end: "End practice",
  pause: "Disable / enable",
};
document.querySelector("#app").innerHTML = `
 <header class="toolbar"><div class="brand"><span class="brand-mark">↔</span><div><h1>Recoil Practice</h1><span class="eyebrow">STRAFE TRAINER / 01</span></div></div><div class="window-actions"><button id="minimise" class="close" title="Minimise to the taskbar" aria-label="Minimise">${icon("minimise")}</button><button id="quit" class="close" title="Close application" aria-label="Close application">${icon("close")}</button></div></header>
 <main class="controls">
  <section class="weapon-bar">
   <button id="weapon-button" class="weapon-button" aria-haspopup="listbox" aria-expanded="false"><img id="weapon-icon" alt="" hidden><span class="weapon-text"><small>WEAPON</small><strong id="weapon-label"></strong></span>${icon("chevron")}</button>
   <div id="modes" class="modes" role="radiogroup" aria-label="Firing mode"></div>
   <div id="weapon-menu" class="weapon-menu" role="listbox" aria-label="Weapon" hidden></div>
  </section>
  <div class="cards">
   ${card("arrows", "Arrows", "arrows-toggle", slider("gap") + slider("arrowSize"))}
   ${card("timeline", "Timeline", "timeline-toggle", slider("timelineOffset") + slider("timelineWidth") + `<label class="field wide" title="Hide the timeline in practice while left-click is held"><span>Hide while shooting</span><input id="timelineIdle-toggle" type="checkbox" class="switch"></label>`)}
   ${card(
     "overlay",
     "Overlay",
     "",
     `<div class="field"><span>Colors</span><select id="theme" aria-label="Color scheme">${Object.entries(
       themes,
     )
       .map(([id, name]) => `<option value="${id}">${name}</option>`)
       .join("")}</select></div>` +
       slider("opacity") +
       `<div class="row" title="The overlay is drawn on your screen as it will appear in practice"><button id="showOverlay" class="mini"></button><button id="move" class="mini" title="Drag the overlay into place">Move</button><button id="centerOverlay" class="mini" title="Put the overlay back on the crosshair">Center</button></div>`,
   )}
   ${card("voice", "Voice", "voice-toggle", `<div class="field"><span>Sound</span><select id="voiceStyle" aria-label="Voice sound"><option value="fast">Fast voice</option><option value="natural">Natural voice</option><option value="tones">Tones</option></select></div><div class="field" title="How long before each change of direction its cue starts"><span>Lead</span><small>before each switch</small><span class="number"><input id="voiceLeadMs" type="number" min="0" max="350" step="10" aria-label="Voice lead"><i>ms</i></span></div><label class="field wide" title="Also announce the first strafe of a spray. It cannot be announced ahead of time, since the click is not predictable."><span>Opening cue</span><input id="voiceStart-toggle" type="checkbox" class="switch"></label>`)}
   ${card(
     "keys",
     "Keys",
     "",
     `<div class="binds">${Object.entries(bindings)
       .map(
         ([id, label]) =>
           `<div class="bind"><span>${label}</span><button id="bind-${id}" class="mini key"></button></div>`,
       )
       .join("")}</div>`,
   )}
  </div>
  <div class="actions"><button id="preview" class="secondary">▷ Preview pattern</button><button id="lock" class="primary">Start practice <kbd id="lock-key"></kbd></button></div>
  <p class="helper">Hold left-click in Apex <span class="divider">·</span> <kbd id="help-start"></kbd> <span id="help-toggle"></span> <span class="divider">·</span> <kbd id="help-pause"></kbd> disable / enable</p>
  <p id="error" role="alert" hidden></p>
 </main>
 <footer><span id="status-dot" class="status-dot"></span><span id="status">Connecting…</span><button id="reset" class="reset" title="Put every setting, key and the overlay position back to its default">Reset defaults</button><span id="version" class="version"></span></footer>
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
let weapons = [];
const categories = {
  ar: "Assault rifles",
  smg: "SMGs",
  lmg: "LMGs",
  pistol: "Pistols",
};
function closeMenu() {
  $("weapon-menu").hidden = true;
  $("weapon-button").setAttribute("aria-expanded", "false");
}
function fillWeapons(list) {
  weapons = list;
  const menu = $("weapon-menu");
  menu.replaceChildren();
  let group;
  for (const weapon of list) {
    if (group?.dataset.category !== weapon.category) {
      const heading = document.createElement("h3");
      heading.textContent = categories[weapon.category] || weapon.category;
      group = document.createElement("div");
      group.className = "weapon-group";
      group.dataset.category = weapon.category;
      menu.append(heading, group);
    }
    const option = document.createElement("button");
    option.className = "weapon-option";
    option.setAttribute("role", "option");
    option.dataset.weapon = weapon.id;
    if (icons[weapon.id]) {
      const image = document.createElement("img");
      image.src = icons[weapon.id];
      image.alt = "";
      option.append(image);
    }
    option.append(weapon.name);
    option.onclick = () => {
      closeMenu();
      if (weapon.id === localSettings.weaponId) return;
      change("weaponId", weapon.id);
      change("modeId", weapon.modes[0].id);
    };
    group.append(option);
  }
  if (state) render(state);
}
function syncWeapon(config) {
  const weapon = weapons.find((w) => w.id === config.weaponId);
  if (!weapon) return;
  $("weapon-label").textContent = weapon.name;
  $("weapon-icon").hidden = !icons[weapon.id];
  if (icons[weapon.id]) $("weapon-icon").src = icons[weapon.id];
  for (const option of $("weapon-menu").querySelectorAll(".weapon-option"))
    option.setAttribute("aria-selected", option.dataset.weapon === weapon.id);
  const modes = $("modes");
  if (modes.dataset.weapon !== weapon.id) {
    modes.dataset.weapon = weapon.id;
    modes.replaceChildren(
      ...weapon.modes.map((mode) => {
        const button = document.createElement("button");
        button.setAttribute("role", "radio");
        button.dataset.mode = mode.id;
        button.textContent = mode.name;
        button.onclick = () => change("modeId", mode.id);
        return button;
      }),
    );
  }
  modes.hidden = weapon.modes.length < 2;
  for (const button of modes.children) {
    const on = button.dataset.mode === config.modeId;
    button.classList.toggle("active", on);
    button.setAttribute("aria-checked", on);
  }
}
function render(s) {
  state = s;
  if (!pending && !saving) localSettings = { ...s.settings };
  const config = localSettings || s.settings;
  document.documentElement.dataset.theme = config.theme;
  $("theme").value = config.theme;
  for (const id in sliders) {
    for (const input of [$(id), $(`${id}-number`)])
      if (document.activeElement !== input) input.value = config[id];
    const [, min, max] = sliders[id];
    $(id).style.setProperty(
      "--p",
      `${((config[id] - min) / (max - min)) * 100}%`,
    );
  }
  if (document.activeElement !== $("voiceLeadMs"))
    $("voiceLeadMs").value = config.voiceLeadMs;
  $("arrows-toggle").checked = config.arrows;
  $("timeline-toggle").checked = config.timeline;
  $("voice-toggle").checked = config.voice;
  $("voiceStyle").value = config.voiceStyle;
  $("voiceStart-toggle").checked = config.voiceStart;
  $("timelineIdle-toggle").checked = config.timelineIdle;
  // A switched-off category keeps its settings but fades the whole card.
  for (const [name, on] of [
    ["arrows", config.arrows],
    ["timeline", config.timeline],
    ["voice", config.voice],
  ]) {
    document.querySelector(`.card.${name}`).classList.toggle("off", !on);
    for (const control of document.querySelectorAll(
      `.card.${name} > :not(h2) :is(input, select)`,
    ))
      control.disabled = !on;
  }
  $("version").textContent = `${s.weapon} · v0.1`;
  syncWeapon(config);
  for (const id in bindings) {
    $(`bind-${id}`).textContent =
      s.binding === id ? "press a key…" : s[`${id}Key`] || "?";
    $(`bind-${id}`).classList.toggle("active", s.binding === id);
  }
  $("lock-key").textContent = s.startKey;
  $("help-start").textContent = s.startKey;
  $("help-toggle").textContent =
    s.startKey === s.endKey ? "edit / practice" : `practice, ${s.endKey} edit`;
  $("help-pause").textContent = s.pauseKey;
  $("preview").textContent =
    s.running && s.preview ? "■ Stop preview" : "▷ Preview pattern";
  $("showOverlay").textContent = s.shown ? "Hide" : "Show";
  $("move").textContent = s.moving ? "Done" : "Move";
  $("move").classList.toggle("active", s.moving);
  $("lock").disabled = !s.inputReady || saving || pending;
  let status = !native
    ? "Browser preview · global input unavailable"
    : !s.inputReady
      ? "Input unavailable"
      : s.binding
        ? "Press an unused key · Esc cancels"
        : !s.armed
          ? `Paused · ${s.pauseKey} to resume`
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
const clamped = (input) =>
  Math.max(
    Number(input.min),
    Math.min(Number(input.max), Math.round(Number(input.value)) || 0),
  );
for (const key in sliders) {
  $(key).addEventListener("input", (e) => change(key, Number(e.target.value)));
  // Typed values apply once complete, so a half-typed number is not clamped.
  $(`${key}-number`).addEventListener("change", (e) => {
    e.target.value = clamped(e.target);
    change(key, Number(e.target.value));
  });
}
$("voiceLeadMs").addEventListener("change", (e) => {
  e.target.value = clamped(e.target);
  change("voiceLeadMs", Number(e.target.value));
});
for (const [id, key] of [
  ["arrows-toggle", "arrows"],
  ["timeline-toggle", "timeline"],
  ["timelineIdle-toggle", "timelineIdle"],
  ["voiceStart-toggle", "voiceStart"],
  ["voice-toggle", "voice"],
])
  $(id).addEventListener("change", (e) => change(key, e.target.checked));
$("voiceStyle").addEventListener("change", (e) =>
  change("voiceStyle", e.target.value),
);
$("theme").addEventListener("change", (e) => change("theme", e.target.value));
$("weapon-button").onclick = () => {
  const open = $("weapon-menu").hidden;
  $("weapon-menu").hidden = !open;
  $("weapon-button").setAttribute("aria-expanded", open);
};
document.addEventListener("click", (e) => {
  if (!e.target.closest(".weapon-bar")) closeMenu();
});
// Flushes pending settings, then runs a command that returns the new state.
const command = (name) => async () => {
  try {
    await flushSettings();
    render(await api[name]());
  } catch (e) {
    error(e);
  }
};
$("preview").onclick = async () => {
  try {
    await flushSettings();
    if (state.running && state.preview) await api.StopPreview();
    else render(await api.Preview());
  } catch (e) {
    error(e);
  }
};
$("lock").onclick = command("ToggleMode");
$("move").onclick = command("ToggleMove");
$("showOverlay").onclick = command("ToggleOverlay");
$("centerOverlay").onclick = command("CenterOverlay");
for (const id in bindings)
  $(`bind-${id}`).onclick = (e) => {
    e.target.blur(); // so Space or Enter can be bound without re-clicking
    api.BindKey(id).then(render).catch(error);
  };
$("minimise").onclick = minimise;
// Resetting discards everything, so it asks for a second click first.
let resetArmed;
function armReset(on) {
  clearTimeout(resetArmed);
  resetArmed = on && setTimeout(() => armReset(false), 3000);
  $("reset").textContent = on ? "Click again to reset" : "Reset defaults";
  $("reset").classList.toggle("active", Boolean(on));
}
$("reset").onclick = async () => {
  if (!resetArmed) return armReset(true);
  armReset(false);
  clearTimeout(debounce);
  pending = false; // unsaved edits are being reset too
  try {
    if (savePromise) await savePromise;
    render(await api.ResetDefaults());
  } catch (e) {
    error(e);
  }
};
$("quit").onclick = () => api.Quit().catch(error);
document.addEventListener("keydown", (e) => {
  if (e.code !== "Escape") return;
  closeMenu();
  if (state?.preview) api.StopPreview().catch(error);
});
onState(render);
api.GetState().then(render).catch(error);
api.Weapons().then(fillWeapons).catch(error);
