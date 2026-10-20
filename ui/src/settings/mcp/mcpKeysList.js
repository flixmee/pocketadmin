import { openMcpKeyTokenModal } from "./mcpKeyTokenModal";
import { openMcpKeyUpsertModal } from "./mcpKeyUpsertModal";
import { isMcpKeyExpired, mcpPermissionOptions } from "./mcpUtils";

export function mcpKeysList(propsArg = {}) {
    const props = store({
        reset: null,
    });

    const watchers = app.utils.extendStore(props, propsArg);

    const data = store({
        isLoading: false,
        isBusy: {},
        keys: [],
        collectionNames: {},
    });

    async function loadKeys() {
        data.isLoading = true;

        try {
            const [result, collections] = await Promise.all([
                app.pb.send("/api/mcp/keys", { requestKey: "mcpKeysList" }),
                app.pb.collections.getFullList({
                    fields: "id,name",
                    requestKey: "mcpKeysList_collections",
                }),
            ]);

            data.keys = result.items || [];
            data.collectionNames = Object.fromEntries(collections.map((c) => [c.id, c.name]));
            data.isLoading = false;
        } catch (err) {
            if (!err.isAbort) {
                app.checkApiError(err);
                data.isLoading = false;
            }
        }
    }

    async function runBusy(key, action) {
        if (data.isBusy[key.id]) {
            return;
        }

        data.isBusy[key.id] = true;

        try {
            await action();
        } catch (err) {
            if (!err.isAbort) {
                app.checkApiError(err);
            }
        }

        delete data.isBusy[key.id];
    }

    function toggleActive(key) {
        return runBusy(key, async () => {
            await app.pb.send(`/api/mcp/keys/${encodeURIComponent(key.id)}`, {
                method: "PATCH",
                body: { active: !key.active },
                requestKey: null,
            });
            app.toasts.success(!key.active ? `Enabled "${key.name}".` : `Disabled "${key.name}".`);
            loadKeys();
        });
    }

    function confirmRegenerate(key) {
        app.modals.confirm(
            `Regenerate "${key.name}"? Clients using the current key will stop working immediately.`,
            () =>
                runBusy(key, async () => {
                    const result = await app.pb.send(`/api/mcp/keys/${encodeURIComponent(key.id)}/regenerate`, {
                        method: "POST",
                        requestKey: null,
                    });
                    loadKeys();
                    openMcpKeyTokenModal(result.key, result.token);
                }),
        );
    }

    function confirmDelete(key) {
        app.modals.confirm(
            `Do you really want to delete "${key.name}"? Clients using it will lose access immediately.`,
            () =>
                runBusy(key, async () => {
                    await app.pb.send(`/api/mcp/keys/${encodeURIComponent(key.id)}`, {
                        method: "DELETE",
                        requestKey: null,
                    });
                    app.toasts.success(`Deleted "${key.name}".`);
                    loadKeys();
                }),
        );
    }

    function statusLabel(key) {
        if (!key.active) {
            return t.span({ className: "label sm" }, "Disabled");
        }
        if (isMcpKeyExpired(key)) {
            return t.span({ className: "label sm danger" }, "Expired");
        }
        return t.span({ className: "label sm success" }, "Active");
    }

    function collectionsSummary(key) {
        if (!key.collections?.length) {
            return "All collections";
        }

        return key.collections.map((id) => data.collectionNames[id] || id).join(", ");
    }

    function permissionsSummary(key) {
        if (!key.permissions?.length) {
            return "No permissions";
        }

        return mcpPermissionOptions
            .filter((opt) => key.permissions.includes(opt.value))
            .map((opt) => opt.label)
            .join(", ");
    }

    return t.div(
        {
            pbEvent: "mcpKeysList",
            className: "list mcp-keys-list",
            onmount: () => {
                watchers.push(
                    watch(() => props.reset, () => {
                        loadKeys();
                    }),
                );
            },
            onunmount: () => {
                watchers.forEach((w) => w?.unwatch());
            },
        },
        () => {
            if (!data.isLoading || data.keys.length) {
                return;
            }

            const skeletons = [];
            for (let i = 0; i < 3; i++) {
                skeletons.push(
                    t.div({ rid: "skeleton_" + i, className: "list-item" }, t.div({ className: "skeleton-loader" })),
                );
            }
            return skeletons;
        },
        t.div(
            {
                hidden: () => data.isLoading || data.keys.length,
                className: "list-item",
            },
            t.div({ className: "content block txt-hint" }, "No MCP API keys yet."),
        ),
        () => {
            return data.keys.map((key) => {
                return t.div(
                    { className: () => `list-item mcp-key-item ${data.isLoading ? "faded" : ""}` },
                    t.i({ className: "ri-key-2-line mcp-key-icon", ariaHidden: true }),
                    t.div(
                        { className: "content mcp-key-content" },
                        t.div(
                            { className: "mcp-key-title" },
                            t.span({ className: "txt-bold txt-ellipsis", title: key.name }, key.name),
                            statusLabel(key),
                            t.code({ className: "txt-hint txt-sm" }, `${key.keyPrefix}…`),
                        ),
                        t.div(
                            { className: "mcp-key-meta txt-hint txt-sm" },
                            t.span(
                                { className: "txt-ellipsis", title: permissionsSummary(key) },
                                t.i({ className: "ri-shield-keyhole-line", ariaHidden: true }),
                                " ",
                                permissionsSummary(key),
                            ),
                            t.span(
                                { className: "txt-ellipsis", title: collectionsSummary(key) },
                                t.i({ className: "ri-database-2-line", ariaHidden: true }),
                                " ",
                                collectionsSummary(key),
                            ),
                        ),
                        t.div(
                            { className: "mcp-key-meta txt-hint txt-sm" },
                            t.span(
                                null,
                                "Last used: ",
                                key.lastUsedAt ? app.utils.toLocalDatetime(key.lastUsedAt) : "never",
                            ),
                            t.span(
                                null,
                                "Expires: ",
                                key.expiresAt ? app.utils.toLocalDatetime(key.expiresAt) : "never",
                            ),
                        ),
                    ),
                    t.nav(
                        { hidden: () => data.isLoading, className: "actions" },
                        t.div(
                            { className: "field mcp-key-switch" },
                            t.input({
                                id: "mcp_key_active_" + key.id,
                                type: "checkbox",
                                className: "switch sm",
                                checked: key.active,
                                disabled: () => data.isBusy[key.id],
                                onchange: (e) => {
                                    e.target.checked = key.active; // reflect the actual state until reloaded
                                    toggleActive(key);
                                },
                            }),
                            t.label({
                                htmlFor: "mcp_key_active_" + key.id,
                                ariaLabel: app.attrs.tooltip(key.active ? "Disable" : "Enable"),
                            }),
                        ),
                        t.button(
                            {
                                type: "button",
                                ariaLabel: app.attrs.tooltip("Edit"),
                                className: "btn sm circle secondary transparent",
                                disabled: () => data.isBusy[key.id],
                                onclick: () => openMcpKeyUpsertModal(key, { onsave: loadKeys }),
                            },
                            t.i({ className: "ri-pencil-line", ariaHidden: true }),
                        ),
                        t.button(
                            {
                                type: "button",
                                ariaLabel: app.attrs.tooltip("Regenerate key"),
                                className: "btn sm circle secondary transparent",
                                disabled: () => data.isBusy[key.id],
                                onclick: () => confirmRegenerate(key),
                            },
                            t.i({ className: "ri-refresh-line", ariaHidden: true }),
                        ),
                        t.button(
                            {
                                type: "button",
                                ariaLabel: app.attrs.tooltip("Delete"),
                                className: "btn sm circle danger transparent",
                                disabled: () => data.isBusy[key.id],
                                onclick: () => confirmDelete(key),
                            },
                            t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
                        ),
                    ),
                );
            });
        },
    );
}
