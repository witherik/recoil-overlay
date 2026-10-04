import { EventsOn } from "../wailsjs/runtime/runtime";
import leftURL from "../../assets/voice/left.wav?url";
import rightURL from "../../assets/voice/right.wav?url";
export const native = Boolean(window.go?.main?.App);
const phases = [
  { direction: "right", durationMs: 800 },
  { direction: "left", durationMs: 530 },
  { direction: "right", durationMs: 880 },
];
const mock = {
  settings: {
    gap: 100,
    arrowSize: 42,
    opacity: 90,
    timeline: true,
    autoCenter: true,
    timelineOffset: 32,
    voice: true,
    voiceLeadMs: 150,
  },
  phases,
  editing: true,
  armed: true,
  focused: false,
  inputReady: false,
  running: false,
  preview: false,
  held: false,
  elapsedMs: 0,
  totalMs: 2210,
  phase: 0,
  direction: "right",
  runId: 0,
  error: "",
};
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
  UpdateSettings: async (s) => {
    stop();
    mock.settings = { ...s };
    return emit();
  },
  Preview: async () => {
    stop();
    mock.running = true;
    mock.preview = true;
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
        if (mock.settings.voice) {
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
  CenterWindow: async () => {},
  SavePosition: async () => {},
  Quit: async () => {},
};
export const api = native ? window.go.main.App : browserAPI;
export function onState(fn) {
  if (native) return EventsOn("state", fn);
  listener = fn;
  return () => {
    listener = null;
    stop();
  };
}
