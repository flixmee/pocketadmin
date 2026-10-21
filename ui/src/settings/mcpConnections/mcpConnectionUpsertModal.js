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
        url: server?.url || "",
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

        const body = {
            name: data.name,
            url: data.url,
            headers: Object.fromEntries(
                data.headers
                    .filter((row) => row.name.trim())
                    .map((row) => [row.name.trim(), row.value]),
            ),
            active: data.active,
        };
        if (data.authToken !== null) {
            body.authToken = data.authToken;
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
        } catch (err) {
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
                            placeholder: "e.g. Web search",
                            value: () => data.name,
                            oninput: (e) => (data.name = e.target.value),
                        }),
                    ),
                ),
                t.div(
                    { className: "col-lg-12" },
                    t.div(
                        { className: "field" },
                        t.label({ htmlFor: uniqueId + "_url" }, "Server URL"),
                        t.input({
                            id: uniqueId + "_url",
                            name: "url",
                            type: "url",
                            required: true,
                            placeholder: "https://example.com/mcp",
                            value: () => data.url,
                            oninput: (e) => (data.url = e.target.value),
                        }),
                    ),
                    t.div({ className: "field-help" }, "The Streamable HTTP endpoint of the MCP server."),
                ),
                t.div(
                    { className: "col-lg-12" },
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
                    { className: "col-lg-12" },
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
