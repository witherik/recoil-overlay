export namespace main {
	
	export class Settings {
	    weaponId: string;
	    modeId: string;
	    gap: number;
	    arrowSize: number;
	    opacity: number;
	    theme: string;
	    arrows: boolean;
	    timeline: boolean;
	    timelineIdle: boolean;
	    offsetX: number;
	    offsetY: number;
	    timelineOffset: number;
	    timelineWidth: number;
	    leftKey: number;
	    rightKey: number;
	    startKey: number;
	    endKey: number;
	    pauseKey: number;
	    voice: boolean;
	    voiceStyle: string;
	    voiceStart: boolean;
	    voiceLeadMs: number;
	    x: number;
	    y: number;
	    positioned: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.weaponId = source["weaponId"];
	        this.modeId = source["modeId"];
	        this.gap = source["gap"];
	        this.arrowSize = source["arrowSize"];
	        this.opacity = source["opacity"];
	        this.theme = source["theme"];
	        this.arrows = source["arrows"];
	        this.timeline = source["timeline"];
	        this.timelineIdle = source["timelineIdle"];
	        this.offsetX = source["offsetX"];
	        this.offsetY = source["offsetY"];
	        this.timelineOffset = source["timelineOffset"];
	        this.timelineWidth = source["timelineWidth"];
	        this.leftKey = source["leftKey"];
	        this.rightKey = source["rightKey"];
	        this.startKey = source["startKey"];
	        this.endKey = source["endKey"];
	        this.pauseKey = source["pauseKey"];
	        this.voice = source["voice"];
	        this.voiceStyle = source["voiceStyle"];
	        this.voiceStart = source["voiceStart"];
	        this.voiceLeadMs = source["voiceLeadMs"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.positioned = source["positioned"];
	    }
	}
	export class Snapshot {
	    settings: Settings;
	    phases: pattern.Phase[];
	    weapon: string;
	    mode: string;
	    editing: boolean;
	    armed: boolean;
	    focused: boolean;
	    inputReady: boolean;
	    running: boolean;
	    preview: boolean;
	    moving: boolean;
	    shown: boolean;
	    held: boolean;
	    elapsedMs: number;
	    totalMs: number;
	    phase: number;
	    direction: string;
	    runId: number;
	    error: string;
	    seq: number;
	    player: pattern.Segment[];
	    playerEndMs: number;
	    score?: pattern.Score;
	    leftKey: string;
	    rightKey: string;
	    startKey: string;
	    endKey: string;
	    pauseKey: string;
	    binding: string;
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.settings = this.convertValues(source["settings"], Settings);
	        this.phases = this.convertValues(source["phases"], pattern.Phase);
	        this.weapon = source["weapon"];
	        this.mode = source["mode"];
	        this.editing = source["editing"];
	        this.armed = source["armed"];
	        this.focused = source["focused"];
	        this.inputReady = source["inputReady"];
	        this.running = source["running"];
	        this.preview = source["preview"];
	        this.moving = source["moving"];
	        this.shown = source["shown"];
	        this.held = source["held"];
	        this.elapsedMs = source["elapsedMs"];
	        this.totalMs = source["totalMs"];
	        this.phase = source["phase"];
	        this.direction = source["direction"];
	        this.runId = source["runId"];
	        this.error = source["error"];
	        this.seq = source["seq"];
	        this.player = this.convertValues(source["player"], pattern.Segment);
	        this.playerEndMs = source["playerEndMs"];
	        this.score = this.convertValues(source["score"], pattern.Score);
	        this.leftKey = source["leftKey"];
	        this.rightKey = source["rightKey"];
	        this.startKey = source["startKey"];
	        this.endKey = source["endKey"];
	        this.pauseKey = source["pauseKey"];
	        this.binding = source["binding"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace pattern {
	
	export class Change {
	    atMs: number;
	    direction: string;
	    deviationMs: number;
	    missed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Change(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.atMs = source["atMs"];
	        this.direction = source["direction"];
	        this.deviationMs = source["deviationMs"];
	        this.missed = source["missed"];
	    }
	}
	export class Phase {
	    direction: string;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new Phase(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.direction = source["direction"];
	        this.durationMs = source["durationMs"];
	    }
	}
	export class Mode {
	    id: string;
	    name: string;
	    phases: Phase[];
	
	    static createFrom(source: any = {}) {
	        return new Mode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.phases = this.convertValues(source["phases"], Phase);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Score {
	    switches: Change[];
	    totalMs: number;
	    missed: number;
	
	    static createFrom(source: any = {}) {
	        return new Score(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.switches = this.convertValues(source["switches"], Change);
	        this.totalMs = source["totalMs"];
	        this.missed = source["missed"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Segment {
	    startMs: number;
	    endMs: number;
	    direction: string;
	
	    static createFrom(source: any = {}) {
	        return new Segment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.startMs = source["startMs"];
	        this.endMs = source["endMs"];
	        this.direction = source["direction"];
	    }
	}
	export class Weapon {
	    id: string;
	    name: string;
	    category: string;
	    modes: Mode[];
	
	    static createFrom(source: any = {}) {
	        return new Weapon(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.modes = this.convertValues(source["modes"], Mode);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

