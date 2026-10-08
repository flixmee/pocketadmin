import { openAutomationDryRunModal } from "./automationDryRunModal";
import { openAutomationRunPreviewModal } from "./automationRunPreviewModal";

const defaultRunsPageSize = 20;

export function openAutomationRunsModal(automation, settings = {
    onbeforeopen: null,
    onafteropen: null,
    onbeforeclose: null,
    onafterclose: null,
}) {
    const modal = automationRunsModal(automation, settings);
    if (!modal) {
        return;
    }

    document.body.appendChild(modal);
    app.modals.open(modal);
}

function automationRunsModal(automation, settings) {
    if (!automation?.id) {
        app.toasts.error("Failed to load automation runs.");
        return null;
    }

    let modal;

    const data = store({
        isLoading: false,
        isLoadingMore: false,
        isRefreshing: false,
        isClearing: false,
        isReplaying: {},
        hasLoaded: false,
        runs: [],
        offset: 0,
        hasMore: false,
    });

    let realtimeUnsubscribe = null;
    let realtimeRefreshTimer = null;
    let pendingRealtimeRefresh = false;
    let isClosed = false;

    async function loadRuns(reset = false) {
        if (data.isLoading || data.isLoadingMore) {
            if (reset) {
                pendingRealtimeRefresh = true;
            }
            return;
        }

        const offset = reset ? 0 : data.offset;
        if (!reset && !data.hasMore && data.hasLoaded) {
            return;
        }

        if (reset) {
            data.isLoading = true;
            data.isRefreshing = data.hasLoaded;
        } else {
            data.isLoadingMore = true;
        }

        try {
            const suffix = new URLSearchParams({
                limit: defaultRunsPageSize,
                offset: offset,
            }).toString();

            const runs = await app.pb.send(`/api/automations/${automation.id}/runs?${suffix}`, {
                requestKey: `automationRuns_${automation.id}_${offset}`,
            });

            data.runs = reset ? runs : data.runs.concat(runs);
            data.offset = data.runs.length;
            data.hasMore = runs.length === defaultRunsPageSize;
            data.hasLoaded = true;
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        data.isLoading = false;
        data.isLoadingMore = false;
        data.isRefreshing = false;

        if (pendingRealtimeRefresh && !isClosed) {
            pendingRealtimeRefresh = false;
            queueRealtimeRefresh();
        }
    }

    function shouldRefreshForRealtimeEvent(event) {
        const record = event?.record || {};

        return !record.automationRef || record.automationRef === automation.id;
    }

    function queueRealtimeRefresh(event) {
        if (isClosed || !shouldRefreshForRealtimeEvent(event)) {
            return;
        }

        clearTimeout(realtimeRefreshTimer);
        realtimeRefreshTimer = setTimeout(() => {
            loadRuns(true);
        }, 150);
    }

    async function subscribeToRealtime() {
        try {
            const unsubscribe = await app.pb.collection("_automationRuns").subscribe("*", queueRealtimeRefresh);
            if (isClosed) {
                unsubscribe().catch((err) => {
                    console.warn("Failed to unsubscribe from automation runs realtime updates:", err);
                });
                return;
            }

            realtimeUnsubscribe = unsubscribe;
        } catch (err) {
            console.warn("Failed to subscribe to automation runs realtime updates:", err);
        }
    }

    function unsubscribeFromRealtime() {
        isClosed = true;
        pendingRealtimeRefresh = false;
        clearTimeout(realtimeRefreshTimer);
        realtimeRefreshTimer = null;

        if (typeof realtimeUnsubscribe === "function") {
            realtimeUnsubscribe().catch((err) => {
                console.warn("Failed to unsubscribe from automation runs realtime updates:", err);
            });
        }

        realtimeUnsubscribe = null;
    }

    async function rerunAutomationRun(run) {
        if (!automation?.id || !run?.id || data.isReplaying[run.id]) {
            return;
        }

        data.isReplaying[run.id] = true;

        try {
            await app.pb.send(`/api/automations/${automation.id}/runs/${run.id}/rerun`, {
                method: "POST",
            });

            app.toasts.success(`Queued replay for run "${run.id}".`);
            await loadRuns(true);
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        delete data.isReplaying[run.id];
    }

    function previewAutomationRun(run) {
        if (!automation?.id || !run?.id || isRunBusy(run)) {
            return;
        }

        openAutomationDryRunModal(automation, {
            runId: run.id,
            input: run.input,
            title: `Replay preview ${run.id}`,
        });
    }

    async function clearAutomationRuns() {
        if (!automation?.id || data.isClearing || data.runs.length === 0) {
            return;
        }

        data.isClearing = true;

        try {
            await app.pb.send(`/api/automations/${automation.id}/runs`, {
                method: "DELETE",
            });

            app.toasts.success(`Cleared runs for "${automation.name || "Automation"}".`);
            data.runs = [];
            data.offset = 0;
            data.hasMore = false;
            data.hasLoaded = true;
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        data.isClearing = false;
    }

    function confirmClearAutomationRuns() {
        app.modals.confirm(
            `Do you really want to clear all recorded runs for "${automation.name || "Automation"}"?`,
            () => clearAutomationRuns(),
            null,
            { yesButton: "Clear", noButton: "Cancel" },
        );
    }

    function hasReplayInFlight() {
        return Object.keys(data.isReplaying || {}).length > 0;
    }

    function hasBusyAction() {
        return data.isLoading || data.isLoadingMore || data.isClearing || hasReplayInFlight();
    }

    function isRunBusy(run) {
        return data.isLoading || data.isLoadingMore || data.isClearing || !!data.isReplaying[run?.id];
    }

    modal = t.div(
        {
            pbEvent: "automationRunsModal",
            className: "modal popup lg automation-runs-modal",
            onbeforeopen: (el) => {
                isClosed = false;
                loadRuns(true);
                subscribeToRealtime();
                return settings.onbeforeopen?.(el);
            },
            onafteropen: (el) => settings.onafteropen?.(el),
            onbeforeclose: (el) => settings.onbeforeclose?.(el),
            onafterclose: (el) => {
                unsubscribeFromRealtime();
                settings.onafterclose?.(el);
                el?.remove();
            },
        },
        t.header(
            { className: "modal-header" },
            t.h5(
                null,
                t.span({ className: "txt-bold" }, automation.name || "Automation"),
                " runs",
            ),
            t.button(
                {
                    type: "button",
                    className: () => `btn sm circle transparent m-l-auto ${data.isRefreshing ? "loading" : ""}`,
                    disabled: () => hasBusyAction(),
                    ariaLabel: app.attrs.tooltip("Refresh"),
                    onclick: () => loadRuns(true),
                },
                t.i({ className: "ri-refresh-line", ariaHidden: true }),
            ),
        ),
        t.div(
            { className: "modal-content" },
            t.div(
                {
                    hidden: () => !data.isLoading || data.hasLoaded,
                    className: "block txt-center",
                },
                t.span({ className: "loader" }),
            ),
            t.div(
                {
                    hidden: () => data.isLoading || !data.hasLoaded || data.runs.length > 0,
                    className: "content block txt-hint",
                },
                "No runs recorded yet.",
            ),
            t.div(
                {
                    hidden: () => !data.hasLoaded || data.runs.length === 0,
                    className: "list",
                },
                () =>
                    data.runs.map((run, index) => {
                        return t.div(
                            {
                                rid: `${run.id}_${index}`,
                                className: "list-item",
                            },
                            t.i({
                                className: () => `ri-timer-flash-line ${runStatusIconClass(run.status)}`,
                                ariaHidden: true,
                            }),
                            t.div(
                                { className: "content block" },
                                t.div(
                                    { className: "flex gap-5 flex-wrap" },
                                    t.span({ className: "txt-bold txt-code" }, () => run.id),
                                    t.span({ className: () => `label ${runStatusClass(run.status)}` }, () =>
                                        formatRunStatus(run.status)),
                                    t.span({ className: "label" }, () =>
                                        formatTriggerType(run.triggerType)),
                                    () => {
                                        if (!hasStepResults(run)) {
                                            return null;
                                        }

                                        return t.span(
                                            { className: "label info" },
                                            `${run.stepResults.length} step(s)`,
                                        );
                                    },
                                ),
                                t.div(
                                    { className: "txt-sm txt-hint m-t-5" },
                                    () => describeRunTiming(run),
                                ),
                                () => {
                                    const summary = describeRunSummary(run);
                                    if (!summary) {
                                        return null;
                                    }

                                    return t.div(
                                        {
                                            className: () =>
                                                `txt-sm m-t-5 ${run.status === "failed" ? "txt-danger" : "txt-hint"}`,
                                        },
                                        summary,
                                    );
                                },
                            ),
                            t.nav(
                                { className: "actions" },
                                t.button(
                                    {
                                        type: "button",
                                        className: () =>
                                            `btn sm circle secondary transparent ${
                                                data.isReplaying[run.id] ? "loading" : ""
                                            }`,
                                        disabled: () => isRunBusy(run),
                                        ariaLabel: app.attrs.tooltip("Run this trigger again"),
                                        onclick: () => rerunAutomationRun(run),
                                    },
                                    t.i({ className: "ri-repeat-line", ariaHidden: true }),
                                ),
                                t.button(
                                    {
                                        type: "button",
                                        className: "btn sm circle secondary transparent",
                                        disabled: () => isRunBusy(run),
                                        ariaLabel: app.attrs.tooltip("Dry-run this trigger payload"),
                                        onclick: () => previewAutomationRun(run),
                                    },
                                    t.i({ className: "ri-play-circle-line", ariaHidden: true }),
                                ),
                                t.button(
                                    {
                                        type: "button",
                                        className: "btn sm circle secondary transparent",
                                        disabled: () => isRunBusy(run),
                                        ariaLabel: app.attrs.tooltip("View run details"),
                                        onclick: () => openAutomationRunPreviewModal(run),
                                    },
                                    t.i({ className: "ri-eye-line", ariaHidden: true }),
                                ),
                            ),
                        );
                    }),
            ),
        ),
        t.footer(
            { className: "modal-footer" },
            t.button(
                {
                    type: "button",
                    className: "btn transparent m-r-auto",
                    disabled: () => data.isClearing,
                    onclick: () => app.modals.close(modal),
                },
                t.span({ className: "txt" }, "Close"),
            ),
            t.button(
                {
                    type: "button",
                    className: () => `btn danger ${data.isClearing ? "loading" : ""}`,
                    hidden: () => !data.hasLoaded || data.runs.length === 0,
                    disabled: () => hasBusyAction(),
                    onclick: () => confirmClearAutomationRuns(),
                },
                t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
                t.span({ className: "txt" }, "Clear"),
            ),
            t.button(
                {
                    type: "button",
                    className: () => `btn secondary ${data.isLoadingMore ? "loading" : ""}`,
                    hidden: () => !data.hasMore,
                    disabled: () => hasBusyAction(),
                    onclick: () => loadRuns(false),
                },
                t.span({ className: "txt" }, "Load more"),
            ),
        ),
    );

    return modal;
}

function formatTriggerType(triggerType) {
    switch (triggerType) {
        case "manual":
            return "Manual";
        case "webhook":
            return "Webhook";
        case "telegram.message":
            return "On Telegram message";
        case "schedule.cron":
            return "Scheduled cron";
        case "record.beforeCreate":
            return "Before record create";
        case "record.beforeUpdate":
            return "Before record update";
        case "record.create":
            return "Record create";
        case "record.update":
            return "Record update";
        case "record.delete":
            return "Record delete";
        case "i18n.translation_missing":
            return "Translation missing";
        case "i18n.locale_published":
            return "Locale published";
        case "i18n.translation_updated":
            return "Translation updated";
        case "i18n.ai_translation_finished":
            return "AI translation finished";
        default:
            return triggerType || "Unknown trigger";
    }
}

function formatRunStatus(status) {
    switch (status) {
        case "queued":
            return "Queued";
        case "running":
            return "Running";
        case "waiting":
            return "Waiting";
        case "success":
            return "Succeeded";
        case "failed":
            return "Failed";
        default:
            return status || "Unknown";
    }
}

function runStatusClass(status) {
    if (status === "success") {
        return "success";
    }
    if (status === "failed") {
        return "danger";
    }
    if (status === "queued" || status === "running" || status === "waiting") {
        return "warning";
    }

    return "";
}

function runStatusIconClass(status) {
    if (status === "success") {
        return "txt-success";
    }
    if (status === "failed") {
        return "txt-danger";
    }
    if (status === "queued" || status === "running" || status === "waiting") {
        return "txt-warning";
    }

    return "txt-hint";
}

function describeRunTiming(run) {
    if (!run?.started && !run?.finished) {
        return "No timing data";
    }

    const started = run?.started ? app.utils.toLocalDatetime(run.started) : "N/A";
    const finished = run?.finished ? app.utils.toLocalDatetime(run.finished) : "N/A";

    if (run?.started && run?.finished) {
        return `${started} -> ${finished}`;
    }

    return `Started ${started} • Finished ${finished}`;
}

function describeRunSummary(run) {
    if (run?.error) {
        const prefix = hasErrorStepIndex(run) ? `Step ${run.errorStepIndex + 1}: ` : "";
        return prefix + app.utils.truncate(run.error, 240);
    }

    if (hasStepResults(run)) {
        const waitingCount = run.stepResults.filter((result) => result?.status === "waiting").length;
        if (waitingCount > 0) {
            return `${waitingCount} step(s) are waiting to resume.`;
        }

        const stoppedCount = run.stepResults.filter((result) => result?.status === "stopped").length;
        if (stoppedCount > 0) {
            return `${stoppedCount} step(s) stopped the workflow early.`;
        }

        const latest = run.stepResults[run.stepResults.length - 1];
        if (typeof latest?.durationMs === "number") {
            return `Last step finished in ${latest.durationMs}ms.`;
        }
    }

    if (run?.input && typeof run.input === "object") {
        const keys = Object.keys(run.input);
        if (keys.length > 0) {
            return `Trigger payload keys: ${keys.join(", ")}.`;
        }
    }

    return "";
}

function hasStepResults(run) {
    return Array.isArray(run?.stepResults) && run.stepResults.length > 0;
}

function hasErrorStepIndex(run) {
    return typeof run?.errorStepIndex === "number" && run.errorStepIndex >= 0;
}
