import { EventsOn, WindowMinimise } from "../wailsjs/runtime/runtime";
import leftURL from "../../assets/voice/left.wav?url";
import rightURL from "../../assets/voice/right.wav?url";
export const native = Boolean(window.go?.main?.App);

// Outside the Windows app (vite dev, the Playwright tests) there is no Go
// side. This stand-in keeps the settings window working there: it holds the
// settings and can play a preview of one pattern, with nothing behind it.
const phases = [
  { direction: "right", durationMs: 800 },
  { direction: "left", durationMs: 530 },
  { direction: "right", durationMs: 880 },
];
// When each phase begins, and when the last one ends.
const starts = phases.map((_, i) =>
  phases.slice(0, i).reduce((ms, phase) => ms + phase.durationMs, 0),
);
const totalMs = starts.at(-1) + phases.at(-1).durationMs;
const mode = (id, name) => ({ id, name, phases });
const weapons = [
  { id: "havoc", name: "HAVOC", category: "ar" },
  { id: "r301", name: "R-301", category: "ar" },
  { id: "r99", name: "R-99", category: "smg" },
].map((weapon) => ({
  ...weapon,
  modes:
    weapon.id === "havoc"
      ? [mode("normal", "Normal"), mode("turbocharged", "Turbocharged")]
      : [mode("default", "Default")],
}));
const mock = {
  settings: {
    weaponId: "r301",
    modeId: "default",
    gap: 100,
    arrowSize: 42,
    opacity: 90,
    theme: "green",
    arrows: true,
    timeline: true,
    timelineIdle: false,
    timelineOffset: 160,
    timelineWidth: 420,
    voice: true,
    voiceStyle: "fast",
    voiceStart: false,
    voiceLeadMs: 60,
  },
  phases,
  weapon: "R-301",
  mode: "EXPECTED STRAFE",
  editing: true,
  focused: false,
  inputReady: false,
  running: false,
  preview: false,
  moving: false,
  shown: true,
  held: false,
  elapsedMs: 0,
  totalMs,
  phase: 0,
  direction: "right",
  runId: 0,
  error: "",
  player: [],
  playerEndMs: 0,
  score: null,
  leftKey: "A",
  rightKey: "D",
  practiceKey: "F8",
  shootKey: "Left click",
  binding: "",
};
const defaults = structuredClone(mock.settings);
let listener, timer, started, nextCue;
const sounds = { left: new Audio(leftURL), right: new Audio(rightURL) };
function stop() {
  clearInterval(timer);
  Object.values(sounds).forEach((s) => {
    s.pause();
    s.currentTime = 0;
  });
  mock.running = false;
  mock.preview = false;
  mock.elapsedMs = 0;
  mock.direction = "right";
  mock.phase = 0;
}
function emit() {
  listener?.(structuredClone(mock));
  return structuredClone(mock);
}
const browserAPI = {
  GetState: async () => structuredClone(mock),
  Weapons: async () => weapons,
  UpdateSettings: async (s) => {
    stop();
    mock.settings = { ...s };
    mock.weapon = weapons.find((w) => w.id === s.weaponId).name;
    return emit();
  },
  Preview: async () => {
    stop();
    mock.running = true;
    mock.preview = true;
    mock.shown = true;
    mock.runId++;
    started = performance.now();
    nextCue = 0;
    timer = setInterval(() => {
      mock.elapsedMs = Math.min(
        totalMs,
        Math.floor(performance.now() - started),
      );
      const done = mock.elapsedMs >= totalMs;
      // A finished pattern holds its last phase.
      mock.phase = starts.findLastIndex((at) => mock.elapsedMs >= at);
      mock.direction = phases[mock.phase]?.direction || "";
      // Each cue leads its phase; the first cannot, and is opt-in.
      while (
        nextCue < phases.length &&
        mock.elapsedMs >= starts[nextCue] - mock.settings.voiceLeadMs
      ) {
        if (mock.settings.voice && (nextCue > 0 || mock.settings.voiceStart)) {
          const sound = sounds[phases[nextCue].direction];
          sound.currentTime = 0;
          sound.play().catch(() => {});
        }
        nextCue++;
      }
      if (done) {
        clearInterval(timer);
        mock.running = false;
      }
      emit();
    }, 16);
    return emit();
  },
  StopPreview: async () => {
    stop();
    emit();
  },
  ToggleMode: async () => {
    mock.error =
      "Open the Windows app to use global input and click-through mode.";
    return emit();
  },
  BindKey: async (action) => {
    mock.binding = mock.binding === action ? "" : action;
    return emit();
  },
  ToggleMove: async () => {
    mock.moving = !mock.moving;
    return emit();
  },
  ToggleOverlay: async () => {
    mock.shown = !mock.shown;
    if (!mock.shown) {
      stop();
      mock.moving = false;
    }
    return emit();
  },
  ResetDefaults: async () => {
    stop();
    mock.settings = structuredClone(defaults);
    mock.weapon = "R-301";
    return emit();
  },
  CenterOverlay: async () => emit(),
  DismissError: async () => {
    mock.error = "";
    return emit();
  },
  Quit: async () => {},
};
// Minimising keeps the app in the taskbar and hides the overlay preview.
export const minimise = () => native && WindowMinimise();
export const api = native ? window.go.main.App : browserAPI;
export function onState(fn) {
  if (native) return EventsOn("state", fn);
  listener = fn;
  return () => {
    listener = null;
    stop();
  };
}
