import { openMcpKeyTokenModal } from "./mcpKeyTokenModal";
import { mcpPermissionOptions } from "./mcpUtils";

/**
 * Opens the create/edit MCP key modal.
 *
 * @param {Object} [key] Existing key to edit (leave empty to create a new one).
 * @param {Object} [settings]
 */
export function openMcpKeyUpsertModal(key = null, settings = {
    onsave: null,
}) {
    const modal = mcpKeyUpsertModal(key, settings);

    document.body.appendChild(modal);

    app.modals.open(modal);
}

function mcpKeyUpsertModal(key, settings) {
    let modal;

    const uniqueId = "mcp_key_" + app.utils.randomString();

    const isNew = !key?.id;

    const data = store({
        isSubmitting: false,
        collections: [],
        name: key?.name || "",
        permissions: key ? [...(key.permissions || [])] : ["collections.read", "records.read"],
        allowedCollections: [...(key?.collections || [])],
        active: key ? !!key.active : true,
        expiresAt: key?.expiresAt || "",
    });

    async function loadCollections() {
        try {
            const collections = await app.pb.collections.getFullList({
                filter: "system = false",
                sort: "name",
                requestKey: uniqueId + "_collections",
            });
            data.collections = collections.map((c) => ({ id: c.id, name: c.name }));
        } catch (err) {
            if (!err.isAbort) {
                app.checkApiError(err);
            }
        }
    }

    function togglePermission(permission, enabled) {
        const permissions = data.permissions.filter((p) => p != permission);
        if (enabled) {
            permissions.push(permission);
        }
        data.permissions = permissions;
    }

    async function submit() {
        if (data.isSubmitting) {
            return;
        }

        data.isSubmitting = true;

        const body = {
            name: data.name,
            permissions: data.permissions,
            collections: data.allowedCollections,
            active: data.active,
            expiresAt: data.expiresAt,
        };

        try {
            if (isNew) {
                const result = await app.pb.send("/api/mcp/keys", {
                    method: "POST",
                    body,
                    requestKey: uniqueId,
                });

                data.isSubmitting = false;
                settings.onsave?.(result.key);
                app.modals.close(modal);
                openMcpKeyTokenModal(result.key, result.token);
            } else {
                const result = await app.pb.send(`/api/mcp/keys/${encodeURIComponent(key.id)}`, {
                    method: "PATCH",
                    body,
                    requestKey: uniqueId,
                });

                data.isSubmitting = false;
                settings.onsave?.(result);
                app.toasts.success(`Successfully saved "${result.name}".`);
                app.modals.close(modal);
            }
        } catch (err) {
            if (!err.isAbort) {
                app.checkApiError(err);
            }
        }

        data.isSubmitting = false;
    }

    modal = t.div(
        {
            pbEvent: "mcpKeyUpsertModal",
            className: "modal popup mcp-key-upsert-modal",
            onmount: () => {
                loadCollections();
            },
            onbeforeclose: () => !data.isSubmitting,
            onafterclose: (el) => {
                app.store.errors = null;
                el?.remove();
            },
        },
        t.header(
            { className: "modal-header" },
            t.h5({ className: "m-auto txt-center" }, isNew ? "New MCP API key" : "Edit MCP API key"),
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
                            placeholder: "e.g. Claude Desktop",
                            value: () => data.name,
                            oninput: (e) => (data.name = e.target.value),
                        }),
                    ),
                ),
                t.div(
                    { className: "col-lg-12" },
                    t.div({ className: "mcp-section-label" }, "Permissions"),
                    t.div(
                        { className: "mcp-permissions" },
                        mcpPermissionOptions.map((option) => {
                            const inputId = uniqueId + "_perm_" + option.value.replace(".", "_");

                            return t.div(
                                {
                                    className: () =>
                                        `field mcp-permission ${
                                            data.permissions.includes(option.value) ? "active" : ""
                                        }`,
                                },
                                t.input({
                                    id: inputId,
                                    type: "checkbox",
                                    checked: () => data.permissions.includes(option.value),
                                    onchange: (e) => togglePermission(option.value, e.target.checked),
                                }),
                                t.label(
                                    { htmlFor: inputId },
                                    t.span(
                                        { className: "mcp-permission-content" },
                                        t.span({ className: "txt-bold" }, option.label),
                                        t.span({ className: "txt-hint txt-sm" }, option.description),
                                    ),
                                ),
                            );
                        }),
                    ),
                    t.div(
                        {
                            hidden: () => !data.permissions.some((p) => p.endsWith(".write")),
                            className: "field-help txt-warning",
                        },
                        "Write permissions allow the AI client to modify or delete your data.",
                    ),
                ),
                t.div(
                    { className: "col-lg-12" },
                    t.div(
                        { className: "field" },
                        t.label({ htmlFor: uniqueId + "_collections" }, "Allowed collections"),
                        app.components.select({
                            id: uniqueId + "_collections",
                            name: "collections",
                            max: 1000,
                            placeholder: "All non-system collections",
                            options: () => data.collections.map((c) => ({ value: c.id, label: c.name })),
                            value: () => data.allowedCollections,
                            onchange: (opts) => (data.allowedCollections = opts.map((opt) => opt.value)),
                        }),
                    ),
                    t.div(
                        { className: "field-help" },
                        "Leave empty to allow all non-system collections. System collections are never accessible. Keys restricted to specific collections can't create new collections.",
                    ),
                ),
                t.div(
                    { className: "col-lg-6" },
                    t.div(
                        { className: "field" },
                        t.label({ htmlFor: uniqueId + "_expiresAt" }, "Expires at (Local)"),
                        t.input({
                            id: uniqueId + "_expiresAt",
                            name: "expiresAt",
                            type: "datetime-local",
                            step: 1,
                            value: () => app.utils.toDatetimeLocalInputValue(data.expiresAt),
                            onchange: (e) => (data.expiresAt = app.utils.toRFC3339Datetime(e.target.value)),
                        }),
                    ),
                    t.div({ className: "field-help" }, "Leave empty for a key that never expires."),
                ),
                t.div(
                    { className: "col-lg-6 flex" },
                    t.div(
                        { className: "field m-t-sm" },
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
                t.span({ className: "txt" }, isNew ? "Create key" : "Save changes"),
            ),
        ),
    );

    return modal;
}
