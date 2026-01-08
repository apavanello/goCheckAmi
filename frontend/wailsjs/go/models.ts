export namespace main {
	
	export class EC2Instance {
	    name: string;
	    ami: string;
	
	    static createFrom(source: any = {}) {
	        return new EC2Instance(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ami = source["ami"];
	    }
	}
	export class AWSResult {
	    parameters: string[];
	    instances: EC2Instance[];
	
	    static createFrom(source: any = {}) {
	        return new AWSResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.parameters = source["parameters"];
	        this.instances = this.convertValues(source["instances"], EC2Instance);
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

