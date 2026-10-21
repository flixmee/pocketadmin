import { settingsSidebar } from "../settingsSidebar";
import { mcpKeysList } from "./mcpKeysList";
import { openMcpKeyUpsertModal } from "./mcpKeyUpsertModal";
import { mcpClientConfig, mcpEndpointURL } from "./mcpUtils";

const mcpToolsInfo = [
    ["list_collections", "Read collections"],
    ["get_collection", "Read collections"],
    ["create_collection", "Manage collections"],
    ["update_collection", "Manage collections"],
    ["delete_collection", "Manage collections"],
    ["list_records", "Read records"],
    ["get_record", "Read records"],
    ["create_record", "Write records"],
    ["update_record", "Write records"],
    ["delete_record", "Write records"],
    ["list_media", "Read media"],
    ["upload_media", "Upload media"],
];

export function pageMcpSettings(route) {
    app.store.title = "MCP";

    const data = store({
        resetList: null,
    });

    function resetKeysList() {
        data.resetList = Date.now();
    }

    return t.div(
        { pbEvent: "pageMcpSettings", className: "page page-mcp-settings" },
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
                        onclick: () => openMcpKeyUpsertModal(null, { onsave: resetKeysList }),
                    },
                    t.i({ className: "ri-add-line", ariaHidden: true }),
                    t.span({ className: "txt" }, "New API key"),
                ),
            ),
            t.div(
                { className: "wrapper m-b-base" },
                t.p(
                    { className: "txt-hint m-b-base" },
                    "The Model Context Protocol (MCP) endpoint lets AI assistants such as Claude, Cursor or VS Code ",
                    "work with your collections and records. Each client authenticates with its own API key ",
                    "that can be limited to specific permissions and collections.",
                ),
                t.div(
                    { className: "flex gap-10 m-b-sm" },
                    t.div({ className: "txt-lg" }, "API keys"),
                    app.components.refreshButton({
                        className: "btn sm transparent secondary circle",
                        onclick: resetKeysList,
                    }),
                ),
                mcpKeysList({
                    reset: () => data.resetList,
                }),
                t.div({ className: "txt-lg m-t-base m-b-sm" }, "Connect a client"),
                t.div(
                    { className: "mcp-connect-card" },
                    t.div(
                        { className: "field" },
                        t.label(null, "Endpoint (Streamable HTTP)"),
                        t.div(
                            { className: "mcp-token-box" },
                            t.code({ className: "mcp-token-value txt-code" }, mcpEndpointURL()),
                            app.components.copyButton(() => mcpEndpointURL()),
                        ),
                    ),
                    t.p(
                        { className: "txt-hint txt-sm m-t-sm m-b-sm" },
                        "Send the key in the ",
                        t.code(null, "Authorization: Bearer <key>"),
                        " (or ",
                        t.code(null, "X-API-Key"),
                        ") header. Example configuration for clients that support remote MCP servers:",
                    ),
                    app.components.codeBlock({
                        language: "json",
                        value: mcpClientConfig(),
                    }),
                    t.details(
                        { className: "mcp-tools-details m-t-sm" },
                        t.summary({ className: "txt-sm" }, "Available tools"),
                        t.table(
                            { className: "table sm mcp-tools-table" },
                            t.thead(
                                null,
                                t.tr(null, t.th(null, "Tool"), t.th(null, "Required permission")),
                            ),
                            t.tbody(
                                null,
                                mcpToolsInfo.map(([name, permission]) =>
                                    t.tr(
                                        null,
                                        t.td(null, t.code(null, name)),
                                        t.td({ className: "txt-hint" }, permission),
                                    )
                                ),
                            ),
                        ),
                    ),
                ),
            ),
            t.footer({ className: "page-footer" }, app.components.credits()),
        ),
    );
}
