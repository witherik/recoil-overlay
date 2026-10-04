export namespace main {
	
	export class Settings {
	    gap: number;
	    arrowSize: number;
	    opacity: number;
	    timeline: boolean;
	    offsetX: number;
	    offsetY: number;
	    timelineOffset: number;
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
	        this.timeline = source["timeline"];
	        this.offsetX = source["offsetX"];
	        this.offsetY = source["offsetY"];
	        this.timelineOffset = source["timelineOffset"];
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

}

