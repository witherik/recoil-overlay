export namespace main {
	
	export class Settings {
	    gap: number;
	    arrowSize: number;
	    opacity: number;
	    arrows: boolean;
	    timeline: boolean;
	    timelineIdle: boolean;
	    offsetX: number;
	    offsetY: number;
	    timelineOffset: number;
	    leftKey: number;
	    rightKey: number;
	    voice: boolean;
	    voiceLeadMs: number;
	    x: number;
	    y: number;
	    width: number;
	    height: number;
	    positioned: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gap = source["gap"];
	        this.arrowSize = source["arrowSize"];
	        this.opacity = source["opacity"];
	        this.arrows = source["arrows"];
	        this.timeline = source["timeline"];
	        this.timelineIdle = source["timelineIdle"];
	        this.offsetX = source["offsetX"];
	        this.offsetY = source["offsetY"];
	        this.timelineOffset = source["timelineOffset"];
	        this.leftKey = source["leftKey"];
	        this.rightKey = source["rightKey"];
	        this.voice = source["voice"];
	        this.voiceLeadMs = source["voiceLeadMs"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.positioned = source["positioned"];
	    }
	}
	export class Snapshot {
	    settings: Settings;
	    phases: pattern.Phase[];
	    editing: boolean;
	    armed: boolean;
	    focused: boolean;
	    inputReady: boolean;
	    running: boolean;
	    preview: boolean;
	    moving: boolean;
	    held: boolean;
	    elapsedMs: number;
	    totalMs: number;
	    phase: number;
	    direction: string;
	    runId: number;
	    error: string;
	    player: pattern.Segment[];
	    playerEndMs: number;
	    score?: pattern.Score;
	    leftKey: string;
	    rightKey: string;
	    binding: string;
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.settings = this.convertValues(source["settings"], Settings);
	        this.phases = this.convertValues(source["phases"], pattern.Phase);
	        this.editing = source["editing"];
	        this.armed = source["armed"];
	        this.focused = source["focused"];
	        this.inputReady = source["inputReady"];
	        this.running = source["running"];
	        this.preview = source["preview"];
	        this.moving = source["moving"];
	        this.held = source["held"];
	        this.elapsedMs = source["elapsedMs"];
	        this.totalMs = source["totalMs"];
	        this.phase = source["phase"];
	        this.direction = source["direction"];
	        this.runId = source["runId"];
	        this.error = source["error"];
	        this.player = this.convertValues(source["player"], pattern.Segment);
	        this.playerEndMs = source["playerEndMs"];
	        this.score = this.convertValues(source["score"], pattern.Score);
	        this.leftKey = source["leftKey"];
	        this.rightKey = source["rightKey"];
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
	export class Score {
	    switches: Change[];
	    averageMs: number;
	    missed: number;
	
	    static createFrom(source: any = {}) {
	        return new Score(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.switches = this.convertValues(source["switches"], Change);
	        this.averageMs = source["averageMs"];
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

}

