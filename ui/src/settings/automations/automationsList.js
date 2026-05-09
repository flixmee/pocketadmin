import { openAutomationRunsModal } from "./automationRunsList";
import { openAutomationUpsertModal } from "./automationUpsertModal";

const triggerLabels = {
    "manual": "Manual",
    "webhook": "Webhook",
    "schedule.cron": "Scheduled cron",
    "record.create": "Record create",
    "record.update": "Record update",
    "record.delete": "Record delete",
};

export function automationsList(propsArg = {}) {
    const props = store({
        reset: null,
    });

    const watchers = app.utils.extendStore(props, propsArg);

    const data = store({
        isLoading: false,
        isRunning: {},
        isDeleting: {},
        isToggling: {},
        automations: [],
    });

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
        openAutomationUpsertModal(null, {
            onsave: () => loadAutomations(),
        });
    }

    function openEditModal(automation) {
        openAutomationUpsertModal(automation, {
            onsave: () => loadAutomations(),
        });
    }

    function openRunsModal(automation) {
        openAutomationRunsModal(automation);
    }

    return t.div(
        {
            pbEvent: "automationsList",
            className: "list automations-list",
            onmount: () => {
                watchers.push(
                    watch(() => props.reset, () => {
                        loadAutomations();
                    }),
                );
            },
            onunmount: () => {
                watchers.forEach((w) => w?.unwatch());
            },
        },
        t.div(
            { className: "list-content" },
            t.div(
                {
                    hidden: () => !data.isLoading || data.automations.length,
                    className: "list-item",
                },
                t.div({ className: "skeleton-loader" }),
            ),
            t.div(
                {
                    hidden: () => data.isLoading || data.automations.length,
                    className: "list-item",
                },
                t.div(
                    { className: "content block txt-hint" },
                    "No automations defined yet. Create one to start wiring record, webhook, cron, or manual workflows.",
                ),
            ),
            () => {
                return data.automations.map((automation) => {
                    return t.div(
                        { className: () => `list-item ${data.isLoading ? "faded" : ""}` },
                        t.i({
                            className: () => `ri-git-branch-line ${automation.active ? "txt-success" : "txt-hint"}`,
                            ariaHidden: true,
                        }),
                        t.div(
                            { className: "content block" },
                            t.div(
                                { className: "flex flex-wrap gap-5" },
                                t.span({
                                    className: "txt-bold txt-ellipsis",
                                    title: () => automation.name,
                                    textContent: () => automation.name,
                                }),
                                t.span(
                                    { className: () => `label ${automation.active ? "success" : ""}` },
                                    () => automation.active ? "Active" : "Inactive",
                                ),
                                t.span(
                                    { className: () => `label ${runStatusClass(automation.lastRunStatus)}` },
                                    () => formatRunStatus(automation.lastRunStatus),
                                ),
                            ),
                            t.div(
                                { className: "txt-sm txt-hint m-t-5" },
                                t.span(
                                    { className: "txt-code" },
                                    () => triggerLabels[automation.triggerType] || automation.triggerType,
                                ),
                                () => {
                                    const scope = describeAutomationScope(automation);
                                    if (!scope) {
                                        return null;
                                    }

                                    return [
                                        t.span(null, " • "),
                                        t.span(null, scope),
                                    ];
                                },
                                t.span(null, " • "),
                                t.span(
                                    null,
                                    () => `${Array.isArray(automation.steps) ? automation.steps.length : 0} step(s)`,
                                ),
                            ),
                            () => {
                                if (!automation.notes) {
                                    return null;
                                }

                                return t.div(
                                    {
                                        className: "txt-sm txt-hint m-t-5 txt-ellipsis",
                                        title: () => automation.notes,
                                    },
                                    automation.notes,
                                );
                            },
                        ),
                        t.div(
                            { className: "content block min-width" },
                            t.div(
                                { className: "txt-sm txt-hint txt-right" },
                                "Last run",
                            ),
                            () => {
                                if (!automation.lastRunAt) {
                                    return t.div({ className: "txt-sm txt-hint txt-right" }, "Never");
                                }

                                return app.components.formattedDate({
                                    value: automation.lastRunAt,
                                    short: true,
                                });
                            },
                        ),
                        t.nav(
                            {
                                hidden: () => data.isLoading,
                                className: "actions autohide",
                            },
                            t.button(
                                {
                                    type: "button",
                                    ariaLabel: app.attrs.tooltip("Edit"),
                                    className: "btn sm circle secondary transparent",
                                    disabled: () => isBusy(automation),
                                    onclick: () => openEditModal(automation),
                                },
                                t.i({ className: "ri-pencil-line", ariaHidden: true }),
                            ),
                            t.button(
                                {
                                    type: "button",
                                    ariaLabel: app.attrs.tooltip(automation.active ? "Disable" : "Enable"),
                                    className: () =>
                                        `btn sm circle secondary transparent ${
                                            data.isToggling[automation.id] ? "loading" : ""
                                        }`,
                                    disabled: () => isBusy(automation),
                                    onclick: () => toggleAutomation(automation),
                                },
                                t.i({
                                    className: () => automation.active ? "ri-pause-line" : "ri-play-line",
                                    ariaHidden: true,
                                }),
                            ),
                            t.button(
                                {
                                    type: "button",
                                    ariaLabel: app.attrs.tooltip("Run now"),
                                    className: () =>
                                        `btn sm circle secondary transparent ${
                                            data.isRunning[automation.id] ? "loading" : ""
                                        }`,
                                    disabled: () => isBusy(automation),
                                    onclick: () => runAutomation(automation),
                                },
                                t.i({ className: "ri-flashlight-line", ariaHidden: true }),
                            ),
                            t.button(
                                {
                                    type: "button",
                                    ariaLabel: app.attrs.tooltip("Recent runs"),
                                    className: "btn sm circle secondary transparent",
                                    disabled: () => isBusy(automation),
                                    onclick: () => openRunsModal(automation),
                                },
                                t.i({ className: "ri-history-line", ariaHidden: true }),
                            ),
                            t.button(
                                {
                                    type: "button",
                                    ariaLabel: app.attrs.tooltip("Delete"),
                                    className: () =>
                                        `btn sm circle secondary transparent ${
                                            data.isDeleting[automation.id] ? "loading" : ""
                                        }`,
                                    disabled: () => isBusy(automation),
                                    onclick: () => confirmDelete(automation),
                                },
                                t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
                            ),
                        ),
                    );
                });
            },
        ),
        t.div(
            { className: "list-item" },
            t.button(
                {
                    type: "button",
                    className: () => `btn secondary block ${data.isLoading ? "loading" : ""}`,
                    disabled: () => data.isLoading,
                    onclick: openCreateModal,
                },
                t.i({ className: "ri-add-line", ariaHidden: true }),
                t.span({ className: "txt" }, "Create automation"),
            ),
        ),
    );

    function isBusy(automation) {
        return !!(
            data.isDeleting[automation.id]
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
        return automation.id ? `POST /api/automation-webhooks/${automation.id}` : "Missing webhook endpoint";
    }

    if (
        automation.triggerType === "record.create"
        || automation.triggerType === "record.update"
        || automation.triggerType === "record.delete"
    ) {
        const collection = (app.store.collections || []).find((item) => item.id === automation.collectionRef);
        return collection?.name || automation.collectionRef || "Missing collection";
    }

    return "";
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
    if (status === "success") {
        return "Succeeded";
    }
    if (status === "failed") {
        return "Failed";
    }

    return status;
}

function runStatusClass(status) {
    if (status === "success") {
        return "success";
    }
    if (status === "failed") {
        return "danger";
    }
    if (status === "queued" || status === "running") {
        return "warning";
    }

    return "";
}
