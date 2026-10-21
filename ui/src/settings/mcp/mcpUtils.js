export const mcpPermissionOptions = [
    {
        value: "collections.read",
        label: "Read collections",
        description: "List collections and view their schema.",
    },
    {
        value: "collections.write",
        label: "Manage collections",
        description: "Create, update and delete collections.",
    },
    {
        value: "records.read",
        label: "Read records",
        description: "List, filter and view records.",
    },
    {
        value: "records.write",
        label: "Write records",
        description: "Create, update and delete records.",
    },
    {
        value: "media.read",
        label: "Read media",
        description: "Browse and search the media library.",
    },
    {
        value: "media.write",
        label: "Upload media",
        description: "Upload files to the media library (creating folders as needed).",
    },
];

export function mcpEndpointURL() {
    return new URL(app.pb.buildURL("/api/mcp"), window.location.href).href;
}

export function mcpClientConfig(token = "<YOUR_MCP_API_KEY>") {
    return JSON.stringify(
        {
            mcpServers: {
                pocketadmin: {
                    type: "http",
                    url: mcpEndpointURL(),
                    headers: {
                        Authorization: `Bearer ${token}`,
                    },
                },
            },
        },
        null,
        2,
    );
}

export function isMcpKeyExpired(key) {
    return !!key?.expiresAt && new Date(key.expiresAt.replace(" ", "T")) < new Date();
}
