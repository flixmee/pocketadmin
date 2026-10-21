import { settingsSidebar } from "../settingsSidebar";
import { openMcpConnectionUpsertModal } from "./mcpConnectionUpsertModal";
import { connectGoogleAccount } from "./mcpConnectionUtils";

export function pageMcpConnections(route) {
    app.store.title = "MCP connections";

    const data = store({
        isLoading: false,
        isBusy: {},
        servers: [],
        tools: {},
    });

    async function loadServers() {
        data.isLoading = true;

        try {
            const result = await app.pb.send("/api/mcp/servers", { requestKey: "mcpConnectionsList" });
            data.servers = result.items || [];
            data.isLoading = false;
        } catch (err) {
            if (!err.isAbort) {
                app.checkApiError(err);
                data.isLoading = false;
            }
        }
    }

    async function runBusy(server, action) {
        if (data.isBusy[server.id]) {
            return;
        }

        data.isBusy[server.id] = true;

        try {
            await action();
        } catch (err) {
            if (!err.isAbort) {
                app.checkApiError(err);
            }
        }

        delete data.isBusy[server.id];
    }

    function toggleActive(server) {
        return runBusy(server, async () => {
            await app.pb.send(`/api/mcp/servers/${encodeURIComponent(server.id)}`, {
                method: "PATCH",
                body: { active: !server.active },
                requestKey: null,
            });
            app.toasts.success(!server.active ? `Enabled "${server.name}".` : `Disabled "${server.name}".`);
            loadServers();
        });
    }

    function testConnection(server) {
        return runBusy(server, async () => {
            const result = await app.pb.send(`/api/mcp/servers/${encodeURIComponent(server.id)}/tools`, {
                requestKey: null,
            });
            data.tools[server.id] = result.tools || [];
            app.toasts.success(`Connected to "${server.name}" (${data.tools[server.id].length} tools).`);
        });
    }

    function connectAccount(server) {
        connectGoogleAccount(server, { onconnect: loadServers });
    }

    function confirmDisconnect(server) {
        app.modals.confirm(
            `Disconnect ${
                server.account || "the Google account"
            } from "${server.name}"? Automation steps using it will fail until it is connected again.`,
            () =>
                runBusy(server, async () => {
                    await app.pb.send(`/api/mcp/servers/${encodeURIComponent(server.id)}/oauth/disconnect`, {
                        method: "POST",
                        requestKey: null,
                    });
                    app.toasts.success(`Disconnected "${server.name}".`);
                    loadServers();
                }),
        );
    }

    function confirmDelete(server) {
        app.modals.confirm(
            `Do you really want to delete "${server.name}"? Automation steps using it will fail until updated.`,
            () =>
                runBusy(server, async () => {
                    await app.pb.send(`/api/mcp/servers/${encodeURIComponent(server.id)}`, {
                        method: "DELETE",
                        requestKey: null,
                    });
                    app.toasts.success(`Deleted "${server.name}".`);
                    loadServers();
                }),
        );
    }

    loadServers();

    return t.div(
        { pbEvent: "pageMcpConnections", className: "page page-mcp-connections" },
        settingsSidebar(),
        t.div(
            { className: "page-content full-height" },
            t.header(
                { className: "page-header" },
                t.nav(
                    { className: "breadcrumbs" },
                    t.div({ className: "breadcrumb-item" }, "Settings"),
                    t.div({ className: "breadcrumb-item" }, () => app.store.title),
                ),
                t.div({ className: "flex-fill" }),
                t.button(
                    {
                        type: "button",
                        className: "btn",
                        onclick: () => openMcpConnectionUpsertModal(null, { onsave: loadServers }),
                    },
                    t.i({ className: "ri-add-line", ariaHidden: true }),
                    t.span({ className: "txt" }, "New connection"),
                ),
            ),
            t.div(
                { className: "wrapper m-b-base" },
                t.p(
                    { className: "txt-hint m-b-base" },
                    "Connect external MCP servers (Streamable HTTP) or built-in integrations such as Gmail to give the ",
                    t.strong(null, "AI generate"),
                    " automation step access to their tools, e.g. web search, a CRM, your mailbox or another PocketAdmin instance.",
                ),
                t.div(
                    { className: "flex gap-10 m-b-sm" },
                    t.div({ className: "txt-lg" }, "Connections"),
                    app.components.refreshButton({
                        className: "btn sm transparent secondary circle",
                        onclick: loadServers,
                    }),
                ),
                t.div(
                    { className: "list mcp-keys-list mcp-connections-list" },
                    () => {
                        if (!data.isLoading || data.servers.length) {
                            return;
                        }

                        return t.div({ className: "list-item" }, t.div({ className: "skeleton-loader" }));
                    },
                    t.div(
                        {
                            hidden: () => data.isLoading || data.servers.length,
                            className: "list-item",
                        },
                        t.div({ className: "content block txt-hint" }, "No MCP connections yet."),
                    ),
                    () =>
                        data.servers.map((server) =>
                            t.div(
                                { className: () => `list-item mcp-key-item ${data.isLoading ? "faded" : ""}` },
                                t.i({
                                    className: `${
                                        server.kind === "gmail" ? "ri-mail-line" : "ri-plug-line"
                                    } mcp-key-icon`,
                                    ariaHidden: true,
                                }),
                                t.div(
                                    { className: "content mcp-key-content" },
                                    t.div(
                                        { className: "mcp-key-title" },
                                        t.span({ className: "txt-bold txt-ellipsis", title: server.name }, server.name),
                                        server.active
                                            ? t.span({ className: "label sm success" }, "Active")
                                            : t.span({ className: "label sm" }, "Disabled"),
                                    ),
                                    server.kind === "gmail"
                                        ? t.div(
                                            { className: "mcp-key-meta txt-hint txt-sm" },
                                            t.span(null, "Gmail"),
                                            server.connected
                                                ? t.span(
                                                    { className: "txt-ellipsis", title: server.account },
                                                    t.i({
                                                        className: "ri-checkbox-circle-line txt-success",
                                                        ariaHidden: true,
                                                    }),
                                                    " ",
                                                    server.account || "Connected",
                                                )
                                                : t.span(
                                                    { className: "txt-warning" },
                                                    t.i({ className: "ri-error-warning-line", ariaHidden: true }),
                                                    " Not connected",
                                                ),
                                        )
                                        : t.div(
                                            { className: "mcp-key-meta txt-hint txt-sm" },
                                            t.span({ className: "txt-ellipsis", title: server.url }, server.url),
                                            t.span(
                                                null,
                                                t.i({ className: "ri-lock-line", ariaHidden: true }),
                                                " ",
                                                server.hasAuthToken ? "Bearer token" : "No token",
                                            ),
                                        ),
                                    () => {
                                        const tools = data.tools[server.id];
                                        if (!tools) {
                                            return;
                                        }

                                        return t.div(
                                            { className: "mcp-key-meta txt-hint txt-sm" },
                                            t.span(
                                                {
                                                    className: "txt-ellipsis",
                                                    title: tools.map((tool) => tool.name).join(", "),
                                                },
                                                t.i({ className: "ri-tools-line", ariaHidden: true }),
                                                " ",
                                                tools.length ? tools.map((tool) => tool.name).join(", ") : "No tools",
                                            ),
                                        );
                                    },
                                ),
                                t.nav(
                                    { hidden: () => data.isLoading, className: "actions" },
                                    server.kind === "gmail"
                                        ? t.button(
                                            {
                                                type: "button",
                                                className: "btn sm secondary transparent",
                                                disabled: () => data.isBusy[server.id],
                                                onclick: () => connectAccount(server),
                                            },
                                            t.i({ className: "ri-google-line", ariaHidden: true }),
                                            t.span({ className: "txt" }, server.connected ? "Reconnect" : "Connect"),
                                        )
                                        : null,
                                    server.kind === "gmail" && server.connected
                                        ? t.button(
                                            {
                                                type: "button",
                                                ariaLabel: app.attrs.tooltip("Disconnect account"),
                                                className: "btn sm circle secondary transparent",
                                                disabled: () => data.isBusy[server.id],
                                                onclick: () => confirmDisconnect(server),
                                            },
                                            t.i({ className: "ri-link-unlink-m", ariaHidden: true }),
                                        )
                                        : null,
                                    t.div(
                                        { className: "field mcp-key-switch" },
                                        t.input({
                                            id: "mcp_server_active_" + server.id,
                                            type: "checkbox",
                                            className: "switch sm",
                                            checked: server.active,
                                            disabled: () => data.isBusy[server.id],
                                            onchange: (e) => {
                                                e.target.checked = server.active; // reflect the actual state until reloaded
                                                toggleActive(server);
                                            },
                                        }),
                                        t.label({
                                            htmlFor: "mcp_server_active_" + server.id,
                                            ariaLabel: app.attrs.tooltip(server.active ? "Disable" : "Enable"),
                                        }),
                                    ),
                                    t.button(
                                        {
                                            type: "button",
                                            ariaLabel: app.attrs.tooltip("Test connection"),
                                            className: () =>
                                                `btn sm circle secondary transparent ${
                                                    data.isBusy[server.id] ? "loading" : ""
                                                }`,
                                            disabled: () => data.isBusy[server.id],
                                            onclick: () => testConnection(server),
                                        },
                                        t.i({ className: "ri-pulse-line", ariaHidden: true }),
                                    ),
                                    t.button(
                                        {
                                            type: "button",
                                            ariaLabel: app.attrs.tooltip("Edit"),
                                            className: "btn sm circle secondary transparent",
                                            disabled: () => data.isBusy[server.id],
                                            onclick: () =>
                                                openMcpConnectionUpsertModal(server, { onsave: loadServers }),
                                        },
                                        t.i({ className: "ri-pencil-line", ariaHidden: true }),
                                    ),
                                    t.button(
                                        {
                                            type: "button",
                                            ariaLabel: app.attrs.tooltip("Delete"),
                                            className: "btn sm circle danger transparent",
                                            disabled: () => data.isBusy[server.id],
                                            onclick: () => confirmDelete(server),
                                        },
                                        t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
                                    ),
                                ),
                            )
                        ),
                ),
            ),
            t.footer({ className: "page-footer" }, app.components.credits()),
        ),
    );
}
