import { mcpClientConfig } from "./mcpUtils";

/**
 * Opens a modal that displays a newly generated MCP key token.
 * The token is not stored on the server and it is shown only once.
 */
export function openMcpKeyTokenModal(key, token) {
    const modal = mcpKeyTokenModal(key, token);

    document.body.appendChild(modal);

    app.modals.open(modal);
}

function mcpKeyTokenModal(key, token) {
    let modal;

    modal = t.div(
        {
            pbEvent: "mcpKeyTokenModal",
            className: "modal popup mcp-key-token-modal",
            onafterclose: (el) => el?.remove(),
        },
        t.header(
            { className: "modal-header" },
            t.h5({ className: "m-auto txt-center" }, () => `API key "${key.name}"`),
        ),
        t.div(
            { className: "modal-content" },
            t.div(
                { className: "alert warning m-b-sm" },
                t.div(
                    { className: "content" },
                    t.p(null, "Copy the key now. For security reasons it is not stored and won't be shown again."),
                ),
            ),
            t.div(
                { className: "mcp-token-box m-b-base" },
                t.code({ className: "mcp-token-value txt-code" }, token),
                app.components.copyButton(token),
            ),
            t.div(
                { className: "flex gap-5 m-b-xs" },
                t.span({ className: "txt-bold" }, "Example client configuration"),
                app.components.copyButton(() => mcpClientConfig(token)),
            ),
            app.components.codeBlock({
                language: "json",
                value: mcpClientConfig(token),
            }),
        ),
        t.footer(
            { className: "modal-footer" },
            t.button(
                {
                    type: "button",
                    className: "btn m-l-auto",
                    onclick: () => app.modals.close(modal),
                },
                t.span({ className: "txt" }, "Done"),
            ),
        ),
    );

    return modal;
}
