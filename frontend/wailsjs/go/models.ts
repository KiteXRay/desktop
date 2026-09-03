export namespace appscan {
	
	export class AppInfo {
	    name: string;
	    exePath: string;
	    icon?: string;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.exePath = source["exePath"];
	        this.icon = source["icon"];
	        this.description = source["description"];
	    }
	}

}

export namespace main {
	
	export class AppInfoDTO {
	    name: string;
	    version: string;
	    repoUrl: string;
	    os: string;
	    arch: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfoDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.repoUrl = source["repoUrl"];
	        this.os = source["os"];
	        this.arch = source["arch"];
	        this.description = source["description"];
	    }
	}
	export class ConnectionDTO {
	    id: string;
	    label: string;
	    link: string;
	    active: boolean;
	    address: string;
	    port: string;
	    protocol: string;
	    tls: string;
	    flow: string;
	    network: string;
	    security: string;
	    configMap: Record<string, string>;
	    bytesRead: number;
	    bytesWritten: number;
	    totalBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.link = source["link"];
	        this.active = source["active"];
	        this.address = source["address"];
	        this.port = source["port"];
	        this.protocol = source["protocol"];
	        this.tls = source["tls"];
	        this.flow = source["flow"];
	        this.network = source["network"];
	        this.security = source["security"];
	        this.configMap = source["configMap"];
	        this.bytesRead = source["bytesRead"];
	        this.bytesWritten = source["bytesWritten"];
	        this.totalBytes = source["totalBytes"];
	    }
	}
	export class NetworkPrivilegesDTO {
	    hasPrivileges: boolean;
	    os: string;
	    exePath: string;
	    command: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new NetworkPrivilegesDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasPrivileges = source["hasPrivileges"];
	        this.os = source["os"];
	        this.exePath = source["exePath"];
	        this.command = source["command"];
	        this.error = source["error"];
	    }
	}
	export class ProxyEndpointsDTO {
	    socks5Host: string;
	    socks5Port: number;
	    httpHost: string;
	    httpPort: number;
	    socks5Url: string;
	    httpUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyEndpointsDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.socks5Host = source["socks5Host"];
	        this.socks5Port = source["socks5Port"];
	        this.httpHost = source["httpHost"];
	        this.httpPort = source["httpPort"];
	        this.socks5Url = source["socks5Url"];
	        this.httpUrl = source["httpUrl"];
	    }
	}
	export class StatsDTO {
	    id: string;
	    active: boolean;
	    bytesRead: number;
	    bytesWritten: number;
	    totalBytes: number;
	    uploadSpeed: number;
	    downloadSpeed: number;
	    readHistory: number[];
	    writeHistory: number[];
	
	    static createFrom(source: any = {}) {
	        return new StatsDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.active = source["active"];
	        this.bytesRead = source["bytesRead"];
	        this.bytesWritten = source["bytesWritten"];
	        this.totalBytes = source["totalBytes"];
	        this.uploadSpeed = source["uploadSpeed"];
	        this.downloadSpeed = source["downloadSpeed"];
	        this.readHistory = source["readHistory"];
	        this.writeHistory = source["writeHistory"];
	    }
	}

}

export namespace updater {
	
	export class ReleaseInfo {
	    available: boolean;
	    currentVersion: string;
	    latestVersion: string;
	    releaseTitle: string;
	    releaseNotes: string;
	    releaseUrl: string;
	    assetUrl: string;
	    assetName: string;
	    assetSize: number;
	
	    static createFrom(source: any = {}) {
	        return new ReleaseInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.releaseTitle = source["releaseTitle"];
	        this.releaseNotes = source["releaseNotes"];
	        this.releaseUrl = source["releaseUrl"];
	        this.assetUrl = source["assetUrl"];
	        this.assetName = source["assetName"];
	        this.assetSize = source["assetSize"];
	    }
	}

}

