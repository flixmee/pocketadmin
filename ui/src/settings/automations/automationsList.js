import { openAutomationRunsModal } from "./automationRunsList";

const triggerLabels = {
    "manual": "Manual",
    "webhook": "Webhook",
    "schedule.cron": "Scheduled cron",
    "record.beforeCreate": "Before record create",
    "record.beforeUpdate": "Before record update",
    "record.create": "Record create",
    "record.update": "Record update",
    "record.delete": "Record delete",
    "i18n.translation_missing": "Translation missing",
    "i18n.locale_published": "Locale published",
    "i18n.translation_updated": "Translation updated",
    "i18n.ai_translation_finished": "AI translation finished",
};

export function automationsList(propsArg = {}) {
    const props = store({
        reset: null,
    });

    const watchers = app.utils.extendStore(props, propsArg);

    const data = store({
        isLoading: false,
        isRunning: {},
        isPublishing: {},
        isDeleting: {},
        isToggling: {},
        automations: [],
        keyword: "",
        tagFilter: "",
        groupByTag: false,
        get visibleAutomations() {
            const keyword = normalizeSearch(data.keyword);
            const tagFilter = normalizeTag(data.tagFilter);

            return data.automations.filter((automation) => {
                const tag = normalizeTag(automation.tag);
                if (tagFilter && tag !== tagFilter) {
                    return false;
                }

                if (!keyword) {
                    return true;
                }

                return automationSearchText(automation).includes(keyword);
            });
        },
        get hasActiveFilters() {
            return !!data.keyword.trim() || !!normalizeTag(data.tagFilter);
        },
    });

    let realtimeUnsubscribe = null;
    let realtimeRefreshTimer = null;
    let isMounted = false;

    async function loadAutomations() {
        data.isLoading = true;

        try {
            data.automations = await app.pb.send("/api/automations", {
                requestKey: "automationsList.load",
            });
            data.isLoading = false;
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
                data.isLoading = false;
            }
        }
    }

    function queueRealtimeRefresh() {
        clearTimeout(realtimeRefreshTimer);

        realtimeRefreshTimer = setTimeout(() => {
            loadAutomations();
        }, 150);
    }

    async function subscribeToRealtime() {
        try {
            const unsubscribe = await app.pb.collection("_automations").subscribe("*", queueRealtimeRefresh);
            if (!isMounted) {
                unsubscribe().catch((err) => {
                    console.warn("Failed to unsubscribe from automation realtime updates:", err);
                });
                return;
            }

            realtimeUnsubscribe = unsubscribe;
        } catch (err) {
            console.warn("Failed to subscribe to automation realtime updates:", err);
        }
    }

    function unsubscribeFromRealtime() {
        isMounted = false;
        clearTimeout(realtimeRefreshTimer);
        realtimeRefreshTimer = null;

        if (typeof realtimeUnsubscribe === "function") {
            realtimeUnsubscribe().catch((err) => {
                console.warn("Failed to unsubscribe from automation realtime updates:", err);
            });
        }

        realtimeUnsubscribe = null;
    }

    async function toggleAutomation(automation) {
        if (!automation?.id || data.isToggling[automation.id]) {
            return;
        }

        data.isToggling[automation.id] = true;

        try {
            await app.pb.send(`/api/automations/${automation.id}`, {
                method: "PATCH",
                body: { active: !automation.active },
            });

            app.toasts.success(!automation.active ? "Automation enabled." : "Automation disabled.");
            await loadAutomations();
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        delete data.isToggling[automation.id];
    }

    async function runAutomation(automation) {
        if (!automation?.id || data.isRunning[automation.id]) {
            return;
        }

        data.isRunning[automation.id] = true;

        try {
            await app.pb.send(`/api/automations/${automation.id}/run`, {
                method: "POST",
            });
            app.toasts.success(`Triggered "${automation.name}".`);
            await loadAutomations();
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        delete data.isRunning[automation.id];
    }

    async function publishAutomation(automation) {
        if (!automation?.id || data.isPublishing[automation.id]) {
            return;
        }

        data.isPublishing[automation.id] = true;

        try {
            await app.pb.send(`/api/automations/${automation.id}/publish`, {
                method: "POST",
                body: {},
            });
            app.toasts.success(`Published "${automation.name}".`);
            await loadAutomations();
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        delete data.isPublishing[automation.id];
    }

    async function deleteAutomation(automation) {
        if (!automation?.id || data.isDeleting[automation.id]) {
            return;
        }

        data.isDeleting[automation.id] = true;

        try {
            await app.pb.send(`/api/automations/${automation.id}`, {
                method: "DELETE",
            });
            app.toasts.success(`Deleted "${automation.name}".`);
            await loadAutomations();
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        delete data.isDeleting[automation.id];
    }

    function confirmDelete(automation) {
        app.modals.confirm(
            `Do you really want to delete "${automation.name}"?`,
            () => deleteAutomation(automation),
            null,
            { yesButton: "Delete", noButton: "Cancel" },
        );
    }

    function openCreateModal() {
        window.location.hash = "#/automations/new";
    }

    function openEditModal(automation) {
        if (automation?.id) {
            window.location.hash = `#/automations/${automation.id}`;
        }
    }

    function openRunsModal(automation) {
        openAutomationRunsModal(automation);
    }

    function resetListFilters() {
        data.keyword = "";
        data.tagFilter = "";
        data.groupByTag = false;
    }

    function tagOptions() {
        return uniqueAutomationTags(data.automations).map((tag) => ({
            value: tag,
            label: tag,
        }));
    }

    function renderAutomationRow(automation) {
        return t.div(
            { className: () => `al-card-row ${data.isLoading ? "al-faded" : ""}` },
            t.div(
                { className: "al-icon-block" },
                t.i({ className: "ri-flashlight-line", ariaHidden: true }),
            ),
            t.div(
                { className: "al-row-content" },
                t.div(
                    { className: "al-row-top" },
                    t.span({
                        className: "al-row-name",
                        title: () => automation.name,
                        textContent: () => automation.name,
                    }),
                    () => {
                        const tag = normalizeTag(automation.tag);
                        if (!tag) {
                            return null;
                        }

                        return t.span(
                            { className: "al-badge al-badge-tag" },
                            t.i({ className: "ri-price-tag-3-line", ariaHidden: true }),
                            t.span(null, tag),
                        );
                    },
                    t.span(
                        {
                            className: () => `al-badge ${automation.active ? "al-badge-indigo" : "al-badge-muted"}`,
                        },
                        () => automation.active ? "Active" : "Inactive",
                    ),
                    t.span(
                        { className: () => `al-badge ${badgeClass(automation.lastRunStatus)}` },
                        () => formatRunStatus(automation.lastRunStatus),
                    ),
                ),
                t.div(
                    { className: "al-row-meta" },
                    t.span(
                        null,
                        () => triggerLabels[automation.triggerType] || automation.triggerType,
                    ),
                    t.span({ className: "al-meta-dot" }, "·"),
                    t.span(
                        null,
                        () => `${Array.isArray(automation.steps) ? automation.steps.length : 0} step(s)`,
                    ),
                    () => {
                        const scope = describeAutomationScope(automation);
                        if (!scope) return null;
                        return t.span(null, t.span({ className: "al-meta-dot" }, "·"), t.span(null, scope));
                    },
                ),
            ),
            t.div(
                { className: "al-last-run" },
                t.div({ className: "al-last-run-label" }, "Last run"),
                () => {
                    if (!automation.lastRunAt) {
                        return t.div({ className: "al-last-run-value" }, "Never");
                    }
                    const d = new Date(automation.lastRunAt);
                    const dateStr = d.toLocaleDateString("en-US", {
                        month: "short",
                        day: "numeric",
                        year: "numeric",
                    });
                    const timeStr = d.toLocaleTimeString("en-US", {
                        hour: "2-digit",
                        minute: "2-digit",
                        second: "2-digit",
                        hour12: false,
                        timeZoneName: "short",
                    });
                    return t.div(
                        null,
                        t.div({ className: "al-last-run-value" }, dateStr),
                        t.div({ className: "al-last-run-time" }, timeStr),
                    );
                },
            ),
            t.div(
                {
                    hidden: () => data.isLoading,
                    className: "al-row-actions",
                },
                t.button(
                    {
                        type: "button",
                        ariaLabel: app.attrs.tooltip("Run now"),
                        className: () => `al-action-btn ${data.isRunning[automation.id] ? "loading" : ""}`,
                        disabled: () => isBusy(automation),
                        onclick: (e) => {
                            e.stopPropagation();
                            runAutomation(automation);
                        },
                    },
                    t.i({ className: "ri-play-fill", ariaHidden: true }),
                ),
                t.button(
                    {
                        type: "button",
                        ariaLabel: app.attrs.tooltip("Edit"),
                        className: "al-action-btn",
                        disabled: () => isBusy(automation),
                        onclick: (e) => {
                            e.stopPropagation();
                            openEditModal(automation);
                        },
                    },
                    t.i({ className: "ri-pencil-line", ariaHidden: true }),
                ),
                t.button(
                    {
                        type: "button",
                        ariaLabel: app.attrs.tooltip("More"),
                        className: "al-action-btn",
                        disabled: () => isBusy(automation),
                        onclick: (e) => showMoreMenu(e, automation),
                    },
                    t.i({ className: "ri-more-2-fill", ariaHidden: true }),
                ),
            ),
        );
    }

    // More menu popover for a row
    function showMoreMenu(e, automation) {
        e.stopPropagation();

        // Close any existing more-menu
        document.querySelectorAll(".al-more-menu").forEach((el) => el.remove());

        const menu = t.div(
            { className: "al-more-menu" },
            t.button(
                {
                    type: "button",
                    className: "al-more-menu-item",
                    onclick: () => {
                        menu.remove();
                        toggleAutomation(automation);
                    },
                },
                t.i({ className: () => automation.active ? "ri-pause-line" : "ri-play-line", ariaHidden: true }),
                t.span(null, () => automation.active ? "Disable" : "Enable"),
            ),
            t.button(
                {
                    type: "button",
                    className: "al-more-menu-item",
                    onclick: () => {
                        menu.remove();
                        openRunsModal(automation);
                    },
                },
                t.i({ className: "ri-history-line", ariaHidden: true }),
                t.span(null, "Recent runs"),
            ),
            t.button(
                {
                    type: "button",
                    className: "al-more-menu-item",
                    onclick: () => {
                        menu.remove();
                        publishAutomation(automation);
                    },
                },
                t.i({ className: "ri-upload-cloud-line", ariaHidden: true }),
                t.span(null, "Publish version"),
            ),
            t.hr({ className: "al-more-menu-divider" }),
            t.button(
                {
                    type: "button",
                    className: "al-more-menu-item danger",
                    onclick: () => {
                        menu.remove();
                        confirmDelete(automation);
                    },
                },
                t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
                t.span(null, "Delete"),
            ),
        );

        document.body.appendChild(menu);

        // Position near the button
        const btnRect = e.currentTarget.getBoundingClientRect();
        menu.style.top = (btnRect.bottom + 4) + "px";
        menu.style.left = Math.max(0, btnRect.right - 180) + "px";

        // Close on outside click (next tick)
        setTimeout(() => {
            function onBodyClick(evt) {
                if (!menu.contains(evt.target)) {
                    menu.remove();
                    document.removeEventListener("click", onBodyClick, true);
                }
            }
            document.addEventListener("click", onBodyClick, true);
        }, 0);
    }

    return t.div(
        {
            pbEvent: "automationsList",
            className: "al-card-list",
            onmount: () => {
                isMounted = true;
                loadAutomations();
                subscribeToRealtime();
                watchers.push(
                    watch(() => props.reset, () => {
                        loadAutomations();
                    }),
                );
            },
            onunmount: () => {
                unsubscribeFromRealtime();
                watchers.forEach((w) => w?.unwatch());
            },
        },
        t.div(
            {
                hidden: () => !data.automations.length,
                className: "al-list-toolbar",
            },
            t.div(
                { className: "al-search-field" },
                t.i({ className: "ri-search-line", ariaHidden: true }),
                t.input({
                    type: "search",
                    placeholder: "Search automations",
                    value: () => data.keyword,
                    oninput: (e) => (data.keyword = e.target.value),
                }),
            ),
            t.div(
                { className: "al-tag-filter" },
                app.components.select({
                    value: () => data.tagFilter,
                    options: tagOptions,
                    placeholder: "All tags",
                    searchThreshold: 8,
                    disabled: () => tagOptions().length === 0,
                    onchange: (selected) => {
                        data.tagFilter = selected?.[0]?.value || "";
                    },
                }),
            ),
            t.label(
                { className: "al-group-toggle" },
                t.input({
                    type: "checkbox",
                    className: "switch",
                    checked: () => data.groupByTag,
                    onchange: (e) => (data.groupByTag = e.target.checked),
                }),
                t.span({ className: "txt" }, "Group by tag"),
            ),
            t.button(
                {
                    type: "button",
                    className: "btn secondary transparent sm",
                    hidden: () => !data.hasActiveFilters && !data.groupByTag,
                    onclick: resetListFilters,
                },
                t.i({ className: "ri-close-line", ariaHidden: true }),
                t.span({ className: "txt" }, "Clear"),
            ),
        ),
        // Loading skeleton
        t.div(
            {
                hidden: () => !data.isLoading || data.automations.length,
                className: "al-card-row",
            },
            t.div({ className: "skeleton-loader" }),
        ),
        // Empty state
        t.div(
            {
                hidden: () => data.isLoading || data.automations.length,
                className: "al-card-row al-empty-state",
            },
            t.div(
                { className: "al-empty-icon-wrap" },
                t.i({ className: "ri-flashlight-line", ariaHidden: true }),
            ),
            t.div({ className: "al-empty-title" }, "No automations yet"),
            t.div(
                { className: "al-empty-hint" },
                "Create one to start wiring record, webhook, cron, or manual workflows.",
            ),
        ),
        t.div(
            {
                hidden: () => data.isLoading || !data.automations.length || data.visibleAutomations.length,
                className: "al-card-row al-empty-state",
            },
            t.div(
                { className: "al-empty-icon-wrap" },
                t.i({ className: "ri-search-eye-line", ariaHidden: true }),
            ),
            t.div({ className: "al-empty-title" }, "No matching automations"),
            t.div(
                { className: "al-empty-hint" },
                "Try another keyword or tag filter.",
            ),
        ),
        // Automation rows
        () => {
            if (!data.groupByTag) {
                return data.visibleAutomations.map(renderAutomationRow);
            }

            return groupedAutomationsByTag(data.visibleAutomations).map((group) => {
                return t.div(
                    { className: "al-tag-group" },
                    t.div(
                        { className: "al-tag-group-header" },
                        t.i({ className: "ri-price-tag-3-line", ariaHidden: true }),
                        t.span(null, group.tag),
                        t.span({ className: "al-tag-group-count" }, `${group.items.length}`),
                    ),
                    ...group.items.map(renderAutomationRow),
                );
            });
        },
        // Create button footer row
        t.div(
            { className: "al-card-row al-create-row" },
            t.button(
                {
                    type: "button",
                    className: () => `al-create-btn ${data.isLoading ? "loading" : ""}`,
                    disabled: () => data.isLoading,
                    onclick: openCreateModal,
                },
                t.i({ className: "ri-add-line", ariaHidden: true }),
                t.span(null, "Create automation"),
            ),
        ),
    );

    function isBusy(automation) {
        return !!(
            data.isDeleting[automation.id]
            || data.isPublishing[automation.id]
            || data.isRunning[automation.id]
            || data.isToggling[automation.id]
        );
    }
}

function describeAutomationScope(automation) {
    if (automation.triggerType === "schedule.cron") {
        return automation.cronExpr || "Missing cron";
    }

    if (automation.triggerType === "webhook") {
        const method = String(automation.webhookMethod || "POST").trim().toUpperCase() || "POST";
        return automation.id ? `${method} /api/automation-webhooks/${automation.id}` : "Missing webhook endpoint";
    }

    if (
        automation.triggerType === "record.create"
        || automation.triggerType === "record.beforeCreate"
        || automation.triggerType === "record.beforeUpdate"
        || automation.triggerType === "record.update"
        || automation.triggerType === "record.delete"
        || automation.triggerType === "i18n.translation_missing"
        || automation.triggerType === "i18n.locale_published"
        || automation.triggerType === "i18n.translation_updated"
        || automation.triggerType === "i18n.ai_translation_finished"
    ) {
        const collection = (app.store.collections || []).find((item) => item.id === automation.collectionRef);
        return collection?.name || automation.collectionRef || "Missing collection";
    }

    return "";
}

function normalizeTag(value) {
    return String(value || "").trim();
}

function normalizeSearch(value) {
    return normalizeTag(value).toLowerCase();
}

function uniqueAutomationTags(automations) {
    return [...new Set((automations || []).map((automation) => normalizeTag(automation.tag)).filter(Boolean))].sort(
        (a, b) => a.localeCompare(b),
    );
}

function automationSearchText(automation) {
    return normalizeSearch(
        [
            automation.id,
            automation.name,
            automation.tag,
            automation.notes,
            automation.triggerType,
            triggerLabels[automation.triggerType],
            describeAutomationScope(automation),
            formatRunStatus(automation.lastRunStatus),
        ].filter(Boolean).join(" "),
    );
}

function groupedAutomationsByTag(automations) {
    const groups = new Map();

    (automations || []).forEach((automation) => {
        const tag = normalizeTag(automation.tag) || "Untagged";
        if (!groups.has(tag)) {
            groups.set(tag, []);
        }
        groups.get(tag).push(automation);
    });

    return [...groups.entries()]
        .sort(([a], [b]) => {
            if (a === "Untagged") {
                return 1;
            }
            if (b === "Untagged") {
                return -1;
            }
            return a.localeCompare(b);
        })
        .map(([tag, items]) => ({ tag, items }));
}

function formatRunStatus(status) {
    if (!status) {
        return "Never run";
    }

    if (status === "queued") {
        return "Queued";
    }
    if (status === "running") {
        return "Running";
    }
    if (status === "waiting") {
        return "Waiting";
    }
    if (status === "success") {
        return "Succeeded";
    }
    if (status === "failed") {
        return "Failed";
    }

    return status;
}

function badgeClass(status) {
    if (status === "success") {
        return "al-badge-green";
    }
    if (status === "failed") {
        return "al-badge-red";
    }
    if (status === "queued" || status === "running" || status === "waiting") {
        return "al-badge-amber";
    }

    return "al-badge-muted";
}
