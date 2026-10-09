import "./style.css";
import { api, minimise, native, onState } from "./bridge";
// Every colour, shared with the overlay (see theme.go).
import theme from "./theme.json";
const root = document.documentElement;
for (const [name, value] of Object.entries(theme.neutral))
  root.style.setProperty(`--${name}`, value);
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

// The settings, by kind of control. Each key is a field of the Go Settings.
// A slider and a number field for the same setting.
const sliders = {
  gap: ["Spacing", 40, 300, 2, "px"],
  arrowSize: ["Size", 24, 72, 2, "px"],
  // Negative puts the timeline above the arrows.
  timelineOffset: ["Spacing", -800, 800, 2, "px"],
  timelineWidth: ["Width", 280, 800, 4, "px"],
  opacity: ["Opacity", 20, 100, 1, "%"],
};
// Checkboxes, with the id `${key}-toggle`. The first three switch a whole card.
const cardToggles = ["arrows", "timeline", "voice"];
const toggles = [...cardToggles, "timelineIdle", "voiceStart"];
const selects = ["theme", "voiceStyle"];
// The actions a key can be bound to (see BindKey in app.go).
const bindings = {
  left: "Strafe left",
  right: "Strafe right",
  start: "Start practice",
  end: "End practice",
};

// The markup. Rows are flex containers, so nothing depends on the whitespace
// between their children.
const glyphs = {
  minimise: "M6 12h12",
  close: "M6 6l12 12M18 6 6 18",
  chevron: "m6 9 6 6 6-6",
};
const icon = (name) =>
  `<svg class="glyph" viewBox="0 0 24 24" aria-hidden="true"><path d="${glyphs[name]}"/></svg>`;
const slider = (id) => {
  const [label, min, max, step, unit] = sliders[id];
  return `<div class="slider"><span>${label}</span><input id="${id}" type="range" min="${min}" max="${max}" step="${step}" aria-label="${label}"><span class="number"><input id="${id}-number" type="number" min="${min}" max="${max}" step="${step}" aria-label="${label} value"><i>${unit}</i></span></div>`;
};
const select = (id, label, description, options) =>
  `<div class="field"><span>${label}</span><select id="${id}" aria-label="${description}">${options
    .map(([value, name]) => `<option value="${value}">${name}</option>`)
    .join("")}</select></div>`;
const check = (key, label, hint) =>
  `<label class="field wide" title="${hint}"><span>${label}</span><input id="${key}-toggle" type="checkbox"></label>`;
const card = (name, title, body) =>
  `<section class="card ${name}"><h2>${title}${cardToggles.includes(name) ? `<input id="${name}-toggle" type="checkbox" aria-label="${title}">` : ""}</h2><div class="card-body">${body}</div></section>`;
const cards = [
  card("arrows", "Arrows", slider("gap") + slider("arrowSize")),
  card(
    "timeline",
    "Timeline",
    slider("timelineOffset") +
      slider("timelineWidth") +
      check(
        "timelineIdle",
        "Hide while shooting",
        "Hide the timeline in practice while left-click is held",
      ),
  ),
  card(
    "overlay",
    "Overlay",
    select(
      "theme",
      "Colors",
      "Color scheme",
      theme.schemes.map(({ id, name }) => [id, name]),
    ) +
      slider("opacity") +
      `<div class="row" title="The overlay is drawn on your screen as it will appear in practice"><button id="showOverlay"></button><button id="move" title="Drag the overlay into place">Move</button><button id="centerOverlay" title="Put the overlay back on the crosshair">Center</button></div>`,
  ),
  card(
    "voice",
    "Voice",
    select("voiceStyle", "Sound", "Voice sound", [
      ["fast", "Fast voice"],
      ["natural", "Natural voice"],
      ["tones", "Tones"],
    ]) +
      `<div class="field" title="How long before each change of direction its cue starts"><span>Lead</span><small>before each switch</small><span class="number"><input id="voiceLeadMs" type="number" min="0" max="350" step="10" aria-label="Voice lead"><i>ms</i></span></div>` +
      check(
        "voiceStart",
        "Opening cue",
        "Also announce the first strafe of a spray. It cannot be announced ahead of time, since the click is not predictable.",
      ),
  ),
  card(
    "keys",
    "Keys",
    `<div class="binds">${Object.entries(bindings)
      .map(
        ([id, label]) =>
          `<div class="bind"><span>${label}</span><button id="bind-${id}" class="key"></button></div>`,
      )
      .join("")}</div>`,
  ),
];
document.querySelector("#app").innerHTML = `
 <header class="toolbar"><div class="brand"><span class="left">←</span><span class="right">→</span><h1>Recoil Practice</h1><span class="version">v0.2</span></div><div class="window-actions"><button id="minimise" class="icon" title="Minimise to the taskbar" aria-label="Minimise">${icon("minimise")}</button><button id="quit" class="icon" title="Close application" aria-label="Close application">${icon("close")}</button></div></header>
 <main class="controls">
  <section class="weapon-bar">
   <button id="weapon-button" class="weapon-button" aria-haspopup="listbox" aria-expanded="false"><img id="weapon-icon" alt="" hidden><span class="weapon-text"><small>WEAPON</small><strong id="weapon-label"></strong></span>${icon("chevron")}</button>
   <div id="modes" class="modes" role="radiogroup" aria-label="Firing mode"></div>
   <div id="weapon-menu" class="weapon-menu" role="listbox" aria-label="Weapon" hidden></div>
  </section>
  <div class="cards">${cards.join("")}</div>
  <div class="actions"><button id="preview">▷ Preview pattern</button><button id="lock" class="primary">Start practice <kbd id="lock-key"></kbd></button></div>
  <p class="helper">Hold left-click in Apex <span class="divider">·</span> <kbd id="help-start"></kbd> <span id="help-toggle"></span></p>
  <p id="error" role="alert" title="Click to dismiss" hidden></p>
 </main>
 <footer><span id="status-dot" class="status-dot"></span><span id="status">Connecting…</span><button id="reset" class="reset" title="Put every setting, key and the overlay position back to its default">Reset defaults</button></footer>
`;
const $ = (id) => document.getElementById(id);

// The state last received from Go, and the settings as edited here. Edits are
// shown at once and saved shortly after; until then, incoming state does not
// overwrite them.
let state,
  localSettings,
  pending = false, // there are edits not yet sent
  saving = null, // the save in flight, a promise
  debounce;
// An error stays up until it is clicked away: either the one in the state, or
// the failure of a command sent from here.
let failure = "";
function showError() {
  const message = state?.error || failure;
  $("error").hidden = !message;
  $("error").textContent = message;
}
function error(err) {
  failure = String(err);
  showError();
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
// The footer's one line on what the app is doing: the first that applies.
function statusText(s) {
  if (!native) return "Browser preview · global input unavailable";
  if (!s.inputReady) return "Input unavailable";
  if (s.binding) return "Press an unused key · Esc cancels";
  if (s.moving) return "Drag the overlay onto your crosshair";
  if (s.editing) return "Edit mode · adjust your overlay";
  if (!s.focused) return "Waiting for Apex Legends";
  if (s.running) return "Follow the highlighted arrow";
  if (s.held) return "Pattern complete · release to reset";
  return "Ready · hold left-click";
}
function render(s) {
  // State is only sent when it changes, so a late copy of an older one must
  // not replace what is shown.
  if (s.seq < state?.seq) return;
  state = s;
  if (!pending && !saving) localSettings = { ...s.settings };
  const config = localSettings || s.settings;
  const scheme = theme.schemes.find((s) => s.id === config.theme);
  root.dataset.theme = scheme.id;
  root.style.setProperty("--right", scheme.right);
  root.style.setProperty("--left", scheme.left);
  // A field being typed in is left alone.
  for (const id in sliders)
    for (const input of [$(id), $(`${id}-number`)])
      if (document.activeElement !== input) input.value = config[id];
  if (document.activeElement !== $("voiceLeadMs"))
    $("voiceLeadMs").value = config.voiceLeadMs;
  for (const id of selects) $(id).value = config[id];
  for (const key of toggles) $(`${key}-toggle`).checked = config[key];
  // A switched-off category keeps its settings but fades the whole card.
  for (const name of cardToggles) {
    document
      .querySelector(`.card.${name}`)
      .classList.toggle("off", !config[name]);
    for (const control of document.querySelectorAll(
      `.card.${name} > :not(h2) :is(input, select)`,
    ))
      control.disabled = !config[name];
  }
  syncWeapon(config);
  for (const id in bindings) {
    $(`bind-${id}`).textContent =
      s.binding === id ? "press a key…" : s[`${id}Key`] || "?";
    $(`bind-${id}`).classList.toggle("waiting", s.binding === id);
  }
  $("lock-key").textContent = s.startKey;
  $("help-start").textContent = s.startKey;
  $("help-toggle").textContent =
    s.startKey === s.endKey ? "edit / practice" : `practice, ${s.endKey} edit`;
  $("preview").textContent =
    s.running && s.preview ? "■ Stop preview" : "▷ Preview pattern";
  $("showOverlay").textContent = s.shown ? "Hide" : "Show";
  $("move").textContent = s.moving ? "Done" : "Move";
  $("move").classList.toggle("active", s.moving);
  // Not tied to unsaved edits: clicking it saves them first.
  $("lock").disabled = !s.inputReady;
  $("status").textContent = statusText(s);
  $("status-dot").classList.toggle("live", s.inputReady);
  showError();
}
// Sends unsaved edits, one request at a time, until none are left.
async function flushSettings() {
  clearTimeout(debounce);
  while (saving || pending) {
    if (saving) {
      await saving;
      continue;
    }
    pending = false;
    saving = api.UpdateSettings({ ...localSettings });
    try {
      const result = await saving;
      saving = null;
      render(result);
    } catch (e) {
      saving = null;
      error(e);
      throw e;
    }
  }
}
// Records a changed setting. Unless send is false, it is saved, and so shown
// on the overlay, shortly after.
function change(key, value, send = true) {
  localSettings = { ...localSettings, [key]: value };
  pending = true;
  render(state);
  clearTimeout(debounce);
  if (send) debounce = setTimeout(() => flushSettings().catch(error), 180);
}
// Typed values apply once complete, so a half-typed number is not clamped.
function typed(input, key) {
  input.addEventListener("change", () => {
    input.value = Math.max(
      Number(input.min),
      Math.min(Number(input.max), Math.round(Number(input.value)) || 0),
    );
    change(key, Number(input.value));
  });
}
for (const key in sliders) {
  // While a slider is dragged only its number follows; the overlay is updated
  // once, when it is let go.
  $(key).addEventListener("input", (e) =>
    change(key, Number(e.target.value), false),
  );
  $(key).addEventListener("change", (e) => change(key, Number(e.target.value)));
  typed($(`${key}-number`), key);
}
typed($("voiceLeadMs"), "voiceLeadMs");
for (const key of toggles)
  $(`${key}-toggle`).addEventListener("change", (e) =>
    change(key, e.target.checked),
  );
for (const id of selects)
  $(id).addEventListener("change", (e) => change(id, e.target.value));
$("weapon-button").onclick = () => {
  const open = $("weapon-menu").hidden;
  $("weapon-menu").hidden = !open;
  $("weapon-button").setAttribute("aria-expanded", open);
};
document.addEventListener("click", (e) => {
  if (!e.target.closest(".weapon-bar")) closeMenu();
});
// Saves pending settings, runs a command, and shows the state it returns.
async function run(command) {
  try {
    await flushSettings();
    const next = await command();
    if (next) render(next);
  } catch (e) {
    error(e);
  }
}
$("preview").onclick = () =>
  run(() =>
    state.running && state.preview ? api.StopPreview() : api.Preview(),
  );
for (const [id, command] of [
  ["lock", "ToggleMode"],
  ["move", "ToggleMove"],
  ["showOverlay", "ToggleOverlay"],
  ["centerOverlay", "CenterOverlay"],
])
  $(id).onclick = () => run(() => api[command]());
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
  $("reset").classList.toggle("confirm", Boolean(on));
}
$("reset").onclick = async () => {
  if (!resetArmed) return armReset(true);
  armReset(false);
  clearTimeout(debounce);
  pending = false; // unsaved edits are being reset too
  try {
    if (saving) await saving;
    render(await api.ResetDefaults());
  } catch (e) {
    error(e);
  }
};
$("error").onclick = () => {
  failure = "";
  if (state?.error) api.DismissError().then(render).catch(error);
  else showError();
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
