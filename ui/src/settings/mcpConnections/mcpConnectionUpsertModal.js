import { connectGoogleAccount, googleOAuthRedirectURL, mcpConnectionKindOptions } from "./mcpConnectionUtils";

/**
 * Opens the create/edit MCP connection modal.
 *
 * @param {Object} [server] Existing connection to edit (leave empty to create a new one).
 * @param {Object} [settings]
 */
export function openMcpConnectionUpsertModal(server = null, settings = {
    onsave: null,
}) {
    const modal = mcpConnectionUpsertModal(server, settings);

    document.body.appendChild(modal);

    app.modals.open(modal);
}

function createHeaderRow(name = "", value = "") {
    return store({ __id: app.utils.randomString(), name, value });
}

function mcpConnectionUpsertModal(server, settings) {
    let modal;

    const uniqueId = "mcp_server_" + app.utils.randomString();

    const isNew = !server?.id;

    const data = store({
        isSubmitting: false,
        name: server?.name || "",
        kind: server?.kind || "http",
        url: server?.url || "",
        clientId: server?.clientId || "",
        // null = keep the current secret
        clientSecret: isNew ? "" : null,
        hasClientSecret: !!server?.hasClientSecret,
        // null = keep the current token
        authToken: isNew ? "" : null,
        hasAuthToken: !!server?.hasAuthToken,
        headers: Object.entries(server?.headers || {}).map(([name, value]) => createHeaderRow(name, value)),
        active: server ? !!server.active : true,
    });

    async function submit() {
        if (data.isSubmitting) {
            return;
        }

        data.isSubmitting = true;

        const isGmail = data.kind === "gmail";

        // new Gmail connections continue directly with the Google sign-in
        // (the popup must be opened synchronously to avoid popup blockers)
        const continueWithGoogle = isNew && isGmail;
        const popup = continueWithGoogle ? window.open("", "pa_google_oauth", "width=520,height=700") : null;

        const body = {
            name: data.name,
            kind: data.kind,
            active: data.active,
        };
        if (isGmail) {
            body.clientId = data.clientId;
            if (data.clientSecret !== null) {
                body.clientSecret = data.clientSecret;
            }
        } else {
            body.url = data.url;
            body.headers = Object.fromEntries(
                data.headers
                    .filter((row) => row.name.trim())
                    .map((row) => [row.name.trim(), row.value]),
            );
            if (data.authToken !== null) {
                body.authToken = data.authToken;
            }
        }

        try {
            const result = await app.pb.send(
                isNew ? "/api/mcp/servers" : `/api/mcp/servers/${encodeURIComponent(server.id)}`,
                {
                    method: isNew ? "POST" : "PATCH",
                    body,
                    requestKey: uniqueId,
                },
            );

            data.isSubmitting = false;
            settings.onsave?.(result);
            app.toasts.success(`Successfully saved "${result.name}".`);
            app.modals.close(modal);

            if (continueWithGoogle) {
                connectGoogleAccount(result, { popup, onconnect: settings.onsave });
            }
        } catch (err) {
            popup?.close();
            if (!err.isAbort) {
                app.checkApiError(err);
            }
        }

        data.isSubmitting = false;
    }

    modal = t.div(
        {
            pbEvent: "mcpConnectionUpsertModal",
            className: "modal popup mcp-connection-upsert-modal",
            onbeforeclose: () => !data.isSubmitting,
            onafterclose: (el) => {
                app.store.errors = null;
                el?.remove();
            },
        },
        t.header(
            { className: "modal-header" },
            t.h5({ className: "m-auto txt-center" }, isNew ? "New MCP connection" : "Edit MCP connection"),
        ),
        t.form(
            {
                id: uniqueId,
                className: "modal-content",
                autocomplete: "off",
                onsubmit: (e) => {
                    e.preventDefault();
                    submit();
                },
            },
            t.div(
                { className: "grid" },
                t.div(
                    { className: "col-lg-12" },
                    t.div(
                        { className: "field" },
                        t.label({ htmlFor: uniqueId + "_name" }, "Name"),
                        t.input({
                            id: uniqueId + "_name",
                            name: "name",
                            type: "text",
                            required: true,
                            maxLength: 100,
                            placeholder: () => (data.kind === "gmail" ? "e.g. Support inbox" : "e.g. Web search"),
                            value: () => data.name,
                            oninput: (e) => (data.name = e.target.value),
                        }),
                    ),
                ),
                t.div(
                    { className: "col-lg-12" },
                    t.div(
                        { className: "field" },
                        t.label({ htmlFor: uniqueId + "_kind" }, "Type"),
                        app.components.select({
                            id: uniqueId + "_kind",
                            required: true,
                            options: mcpConnectionKindOptions,
                            value: () => data.kind,
                            onchange: (opts) => (data.kind = opts?.[0]?.value || "http"),
                        }),
                    ),
                ),
                t.div(
                    { className: "col-lg-12", hidden: () => data.kind !== "gmail" },
                    t.div(
                        { className: "mcp-gmail-info txt-sm" },
                        t.p(
                            { className: "m-b-xs" },
                            "Built-in Gmail tools (search, read, send & reply) for the AI generate step. ",
                            "Create an OAuth client (type \"Web application\") in Google Cloud Console, enable the Gmail API ",
                            "and add this authorized redirect URI:",
                        ),
                        t.div(
                            { className: "mcp-token-box" },
                            t.code({ className: "mcp-token-value txt-code" }, () => googleOAuthRedirectURL()),
                            app.components.copyButton(() => googleOAuthRedirectURL()),
                        ),
                        t.p(
                            { className: "m-t-xs m-b-0" },
                            "This is the address you're using right now. If you open the admin from a different URL ",
                            "(e.g. another domain), add that callback URL to the Google client too.",
                        ),
                    ),
                ),
                t.div(
                    { className: "col-lg-6", hidden: () => data.kind !== "gmail" },
                    t.div(
                        { className: "field" },
                        t.label({ htmlFor: uniqueId + "_clientId" }, "Google client ID"),
                        t.input({
                            id: uniqueId + "_clientId",
                            name: "clientId",
                            type: "text",
                            placeholder: "Optional",
                            value: () => data.clientId,
                            oninput: (e) => (data.clientId = e.target.value),
                        }),
                    ),
                ),
                t.div(
                    { className: "col-lg-6", hidden: () => data.kind !== "gmail" },
                    t.div(
                        { className: "field" },
                        t.label({ htmlFor: uniqueId + "_clientSecret" }, "Google client secret"),
                        t.input({
                            id: uniqueId + "_clientSecret",
                            name: "clientSecret",
                            type: "password",
                            autocomplete: "new-password",
                            placeholder: () =>
                                data.clientSecret === null && data.hasClientSecret
                                    ? "* * * * * * (unchanged)"
                                    : "Optional",
                            value: () => data.clientSecret || "",
                            oninput: (e) => (data.clientSecret = e.target.value),
                        }),
                    ),
                ),
                t.div(
                    { className: "col-lg-12", hidden: () => data.kind !== "gmail" },
                    t.div(
                        { className: "field-help" },
                        "Leave empty to use the Google client from Settings > Credentials. ",
                        () => (isNew ? "After saving you'll be asked to sign in with the Gmail account." : ""),
                    ),
                ),
                t.div(
                    { className: "col-lg-12", hidden: () => data.kind !== "http" },
                    t.div(
                        { className: "field" },
                        t.label({ htmlFor: uniqueId + "_url" }, "Server URL"),
                        t.input({
                            id: uniqueId + "_url",
                            name: "url",
                            type: "url",
                            required: () => data.kind === "http",
                            placeholder: "https://example.com/mcp",
                            value: () => data.url,
                            oninput: (e) => (data.url = e.target.value),
                        }),
                    ),
                    t.div({ className: "field-help" }, "The Streamable HTTP endpoint of the MCP server."),
                ),
                t.div(
                    { className: "col-lg-12", hidden: () => data.kind !== "http" },
                    t.div(
                        { className: "field" },
                        t.label({ htmlFor: uniqueId + "_authToken" }, "Bearer token"),
                        t.input({
                            id: uniqueId + "_authToken",
                            name: "authToken",
                            type: "password",
                            autocomplete: "new-password",
                            placeholder: () =>
                                data.authToken === null && data.hasAuthToken
                                    ? "* * * * * * (unchanged)"
                                    : "Optional",
                            value: () => data.authToken || "",
                            oninput: (e) => (data.authToken = e.target.value),
                        }),
                    ),
                    t.div(
                        { className: "field-help" },
                        "Sent as ",
                        t.code(null, "Authorization: Bearer <token>"),
                        ". Stored on the server and never shown again.",
                        () =>
                            !isNew && data.hasAuthToken && data.authToken === null
                                ? t.button(
                                    {
                                        type: "button",
                                        className: "link-hint m-l-5",
                                        onclick: () => (data.authToken = ""),
                                    },
                                    "Remove token",
                                )
                                : null,
                    ),
                ),
                t.div(
                    { className: "col-lg-12", hidden: () => data.kind !== "http" },
                    t.div(
                        { className: "flex gap-5 m-b-xs" },
                        t.div({ className: "mcp-section-label m-0" }, "Custom headers"),
                        t.button(
                            {
                                type: "button",
                                className: "btn sm secondary transparent m-l-auto",
                                onclick: () => data.headers.push(createHeaderRow()),
                            },
                            t.i({ className: "ri-add-line", ariaHidden: true }),
                            t.span({ className: "txt" }, "Add header"),
                        ),
                    ),
                    () => {
                        if (!data.headers.length) {
                            return t.div({ className: "txt-hint txt-sm" }, "No custom headers.");
                        }

                        return t.div(
                            { className: "mcp-header-rows" },
                            data.headers.map((row, index) =>
                                t.div(
                                    { rid: row.__id, className: "mcp-header-row" },
                                    t.div(
                                        { className: "field" },
                                        t.input({
                                            type: "text",
                                            placeholder: "X-Api-Key",
                                            ariaLabel: "Header name",
                                            value: () => row.name,
                                            oninput: (e) => (row.name = e.target.value),
                                        }),
                                    ),
                                    t.div(
                                        { className: "field" },
                                        t.input({
                                            type: "text",
                                            placeholder: "Value",
                                            ariaLabel: "Header value",
                                            value: () => row.value,
                                            oninput: (e) => (row.value = e.target.value),
                                        }),
                                    ),
                                    t.button(
                                        {
                                            type: "button",
                                            className: "btn sm circle transparent secondary",
                                            ariaLabel: app.attrs.tooltip("Remove"),
                                            onclick: () => data.headers.splice(index, 1),
                                        },
                                        t.i({ className: "ri-close-line", ariaHidden: true }),
                                    ),
                                )
                            ),
                        );
                    },
                    t.div(
                        { className: "field-help" },
                        "Header values are visible to superusers. Use the bearer token field for secrets when possible.",
                    ),
                ),
                t.div(
                    { className: "col-lg-12" },
                    t.div(
                        { className: "field" },
                        t.input({
                            id: uniqueId + "_active",
                            name: "active",
                            type: "checkbox",
                            className: "switch",
                            checked: () => data.active,
                            onchange: (e) => (data.active = e.target.checked),
                        }),
                        t.label({ htmlFor: uniqueId + "_active" }, "Enabled"),
                    ),
                ),
            ),
        ),
        t.footer(
            { className: "modal-footer" },
            t.button(
                {
                    type: "button",
                    className: "btn transparent m-r-auto",
                    disabled: () => data.isSubmitting,
                    onclick: () => app.modals.close(modal),
                },
                t.span({ className: "txt" }, "Cancel"),
            ),
            t.button(
                {
                    "html-form": uniqueId,
                    type: "submit",
                    className: () => `btn ${data.isSubmitting ? "loading" : ""}`,
                    disabled: () => data.isSubmitting,
                },
                t.span({ className: "txt" }, isNew ? "Create connection" : "Save changes"),
            ),
        ),
    );

    return modal;
}
