import { EventsOn, WindowMinimise } from "../wailsjs/runtime/runtime";
import leftURL from "../../assets/voice/left.wav?url";
import rightURL from "../../assets/voice/right.wav?url";
export const native = Boolean(window.go?.main?.App);
const phases = [
  { direction: "right", durationMs: 800 },
  { direction: "left", durationMs: 530 },
  { direction: "right", durationMs: 880 },
];
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
    theme: "mint",
    arrows: true,
    timeline: true,
    timelineIdle: false,
    timelineOffset: 32,
    timelineWidth: 420,
    voice: true,
    voiceStyle: "fast",
    voiceStart: false,
    voiceLeadMs: 75,
  },
  phases,
  weapon: "R-301",
  mode: "EXPECTED STRAFE",
  editing: true,
  armed: true,
  focused: false,
  inputReady: false,
  running: false,
  preview: false,
  moving: false,
  shown: true,
  held: false,
  elapsedMs: 0,
  totalMs: 2210,
  phase: 0,
  direction: "right",
  runId: 0,
  error: "",
  player: [],
  playerEndMs: 0,
  score: null,
  leftKey: "A",
  rightKey: "D",
  startKey: "F8",
  endKey: "F8",
  pauseKey: "F9",
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
    const cueTimes = [
      0,
      800 - mock.settings.voiceLeadMs,
      1330 - mock.settings.voiceLeadMs,
    ];
    timer = setInterval(() => {
      mock.elapsedMs = Math.min(2210, Math.floor(performance.now() - started));
      mock.phase =
        mock.elapsedMs < 800
          ? 0
          : mock.elapsedMs < 1330
            ? 1
            : mock.elapsedMs < 2210
              ? 2
              : -1;
      mock.direction = phases[mock.phase]?.direction || "";
      while (nextCue < 3 && mock.elapsedMs >= cueTimes[nextCue]) {
        if (mock.settings.voice && (nextCue > 0 || mock.settings.voiceStart)) {
          const sound = sounds[phases[nextCue].direction];
          sound.currentTime = 0;
          sound.play().catch(() => {});
        }
        nextCue++;
      }
      if (mock.elapsedMs >= 2210) {
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
  BindKey: async (side) => {
    mock.binding = mock.binding === side ? "" : side;
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
  SavePosition: async () => {},
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
