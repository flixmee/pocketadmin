window.app = window.app || {};
window.app.modals = window.app.modals || {};

export function openAutomationRunPreviewModal(run, settings = {
    onbeforeopen: null,
    onafteropen: null,
    onbeforeclose: null,
    onafterclose: null,
}) {
    const modal = automationRunPreviewModal(run, settings);
    if (!modal) {
        return;
    }

    document.body.appendChild(modal);
    app.modals.open(modal);
}

window.app.modals.openAutomationRunPreview = openAutomationRunPreviewModal;

function automationRunPreviewModal(run, settings) {
    if (!run?.id) {
        app.toasts.error("Failed to load automation run.");
        return;
    }

    let modal;

    const uniqueId = app.utils.randomString();
    const data = store({
        run,
        get prettyInput() {
            return stringifyJSON(data.run.input);
        },
        get prettyStepResults() {
            return stringifyJSON(data.run.stepResults);
        },
        get stepResults() {
            return Array.isArray(data.run.stepResults) ? data.run.stepResults : [];
        },
    });

    modal = t.div(
        {
            pbEvent: "automationRunPreviewModal",
            className: "modal popup lg automation-run-preview-modal",
            onbeforeopen: (el) => settings.onbeforeopen?.(el),
            onafteropen: (el) => settings.onafteropen?.(el),
            onbeforeclose: (el) => settings.onbeforeclose?.(el),
            onafterclose: (el) => {
                settings.onafterclose?.(el);
                el?.remove();
            },
        },
        t.header(
            { className: "modal-header" },
            t.h5(null, "Automation run"),
            t.button(
                {
                    className: "btn sm circle transparent m-l-auto",
                    title: "More options",
                    "html-popovertarget": uniqueId + "_run_preview_dropdown",
                },
                t.i({ className: "ri-more-line", ariaHidden: true }),
            ),
            t.div(
                {
                    id: uniqueId + "_run_preview_dropdown",
                    className: "dropdown",
                    popover: "auto",
                },
                (el) => {
                    return t.button(
                        {
                            className: "dropdown-item",
                            onclick: () => {
                                app.utils.copyToClipboard(JSON.stringify(data.run, null, 2));
                                app.toasts.success("Automation run copied to clipboard!");
                                el.hidePopover();
                            },
                        },
                        t.i({ className: "ri-braces-line", ariaHidden: true }),
                        t.span({ className: "txt" }, "Copy JSON"),
                    );
                },
            ),
        ),
        t.div(
            { className: "modal-content" },
            t.div(
                { className: "grid" },
                t.div(
                    { className: "col-lg-12" },
                    t.div(
                        { className: "flex gap-5 flex-wrap m-b-sm" },
                        t.span(
                            { className: () => `label ${runStatusClass(data.run.status)}` },
                            () => formatRunStatus(data.run.status),
                        ),
                        t.span({ className: "label" }, () => formatTriggerType(data.run.triggerType)),
                        () => {
                            if (!data.run.error) {
                                return null;
                            }

                            return t.span({ className: "label danger" }, "Error");
                        },
                    ),
                ),
                summaryField("Run ID", () => data.run.id, true),
                summaryDateField("Started", () => data.run.started),
                summaryDateField("Finished", () => data.run.finished),
                summaryField("Error step", () => hasErrorStepIndex(data.run) ? data.run.errorStepIndex : "N/A", false),
                t.div(
                    {
                        className: "col-lg-12",
                        hidden: () => !data.run.error,
                    },
                    t.div(
                        { className: "field" },
                        t.label(null, "Error"),
                        t.div({ className: "label danger block txt-left" }, () => data.run.error),
                    ),
                ),
                t.div(
                    { className: "col-lg-12" },
                    t.div(
                        { className: "field" },
                        t.label(
                            null,
                            `Input payload (${Array.isArray(data.run.input) ? "array" : typeof data.run.input})`,
                        ),
                        app.components.codeBlock({
                            language: "js",
                            value: () => data.prettyInput,
                        }),
                    ),
                ),
                t.div(
                    { className: "col-lg-12" },
                    t.div(
                        { className: "field" },
                        t.label(null, `Step results (${data.stepResults.length})`),
                        () => {
                            if (!data.stepResults.length) {
                                return t.div({ className: "txt-sm txt-hint" }, "No step results recorded.");
                            }

                            return t.div(
                                { className: "list" },
                                () =>
                                    data.stepResults.map((result, index) => {
                                        return t.div(
                                            {
                                                rid: `${data.run.id}_${index}`,
                                                className: "list-item",
                                            },
                                            t.div(
                                                { className: "content block" },
                                                t.div(
                                                    { className: "flex gap-5 flex-wrap" },
                                                    t.span({ className: "txt-bold" }, () =>
                                                        `Step ${(result.index ?? index) + 1}`),
                                                    t.span({ className: "label" }, () =>
                                                        result.type || "unknown"),
                                                    t.span(
                                                        {
                                                            className: () =>
                                                                `label ${stepResultStatusClass(result.status)}`,
                                                        },
                                                        () => formatStepResultStatus(result.status),
                                                    ),
                                                    () => {
                                                        if (typeof result.durationMs !== "number") {
                                                            return null;
                                                        }

                                                        return t.span(
                                                            { className: "label info" },
                                                            `${result.durationMs}ms`,
                                                        );
                                                    },
                                                ),
                                                t.div(
                                                    { className: "txt-sm txt-hint m-t-5" },
                                                    () => formatRunDateInline(result.started, result.finished),
                                                ),
                                                () => {
                                                    if (!result.error) {
                                                        return null;
                                                    }

                                                    return t.div(
                                                        { className: "txt-sm txt-danger m-t-5" },
                                                        result.error,
                                                    );
                                                },
                                            ),
                                        );
                                    }),
                            );
                        },
                    ),
                ),
                t.div(
                    { className: "col-lg-12" },
                    t.div(
                        { className: "field" },
                        t.label(null, "Raw step results JSON"),
                        app.components.codeBlock({
                            language: "js",
                            value: () => data.prettyStepResults,
                        }),
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
                    onclick: () => app.modals.close(modal),
                },
                t.span({ className: "txt" }, "Close"),
            ),
        ),
    );

    return modal;
}

function summaryField(label, value, code = false) {
    return t.div(
        { className: "col-md-6" },
        t.div(
            { className: "field" },
            t.label(null, label),
            t.div(
                { className: () => code ? "txt-code" : "" },
                () => value(),
            ),
        ),
    );
}

function summaryDateField(label, value) {
    return t.div(
        { className: "col-md-6" },
        t.div(
            { className: "field" },
            t.label(null, label),
            () => {
                if (!value()) {
                    return t.span({ className: "txt-hint" }, "N/A");
                }

                return app.components.formattedDate({
                    value,
                    short: false,
                });
            },
        ),
    );
}

function stringifyJSON(value) {
    if (typeof value === "string") {
        try {
            value = JSON.parse(value);
        } catch (_) {
            return value;
        }
    }

    try {
        return JSON.stringify(value ?? null, null, 2);
    } catch (_) {
        return String(value ?? "");
    }
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
    if (status === "queued" || status === "running") {
        return "warning";
    }

    return "";
}

function formatStepResultStatus(status) {
    switch (status) {
        case "success":
            return "Succeeded";
        case "failed":
            return "Failed";
        case "stopped":
            return "Stopped";
        default:
            return status || "Unknown";
    }
}

function stepResultStatusClass(status) {
    if (status === "success") {
        return "success";
    }
    if (status === "failed") {
        return "danger";
    }
    if (status === "stopped") {
        return "warning";
    }

    return "";
}

function hasErrorStepIndex(run) {
    return typeof run?.errorStepIndex === "number" && run.errorStepIndex >= 0;
}

function formatRunDateInline(started, finished) {
    if (!started && !finished) {
        return "No timing data";
    }

    if (started && finished) {
        return `${app.utils.toLocalDatetime(started)} -> ${app.utils.toLocalDatetime(finished)}`;
    }

    return app.utils.toLocalDatetime(started || finished);
}
