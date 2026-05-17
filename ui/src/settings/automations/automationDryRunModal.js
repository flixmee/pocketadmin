export function openAutomationDryRunModal(automation) {
    const modal = automationDryRunModal(automation);
    if (!modal) {
        return;
    }

    document.body.appendChild(modal);
    app.modals.open(modal);
}

function automationDryRunModal(automation) {
    if (!automation?.id) {
        app.toasts.error("Save the automation before running a preview.");
        return;
    }

    let modal;

    const data = store({
        isRunning: false,
        inputText: "{}",
        result: null,
        get prettyResult() {
            return stringifyJSON(data.result || {});
        },
        get stepResults() {
            return Array.isArray(data.result?.stepResults) ? data.result.stepResults : [];
        },
    });

    async function runPreview() {
        if (data.isRunning) {
            return;
        }

        let input;
        try {
            input = parseJSONObject(data.inputText);
        } catch (err) {
            app.toasts.error(err.message);
            return;
        }

        data.isRunning = true;
        data.result = null;

        try {
            data.result = await app.pb.send(`/api/automations/${automation.id}/dry-run`, {
                method: "POST",
                body: input,
            });
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        data.isRunning = false;
    }

    modal = t.div(
        {
            pbEvent: "automationDryRunModal",
            className: "modal popup lg automation-dry-run-modal",
            onafterclose: (el) => el?.remove(),
        },
        t.header(
            { className: "modal-header" },
            t.h5(null, "Dry-run preview"),
        ),
        t.div(
            { className: "modal-content" },
            t.div(
                { className: "grid" },
                t.div(
                    { className: "col-lg-12" },
                    t.div(
                        { className: "field" },
                        t.label(null, "Input JSON"),
                        t.textarea({
                            rows: 8,
                            spellcheck: false,
                            value: () => data.inputText,
                            oninput: (e) => (data.inputText = e.target.value),
                        }),
                    ),
                    t.div(
                        { className: "field-help" },
                        "Preview executes through the dry-run runtime, so side-effecting steps are simulated.",
                    ),
                ),
                t.div(
                    { className: "col-lg-12", hidden: () => !data.result },
                    t.div(
                        { className: "flex gap-5 flex-wrap m-b-sm" },
                        t.span(
                            { className: () => `label ${runStatusClass(data.result?.status)}` },
                            () => formatRunStatus(data.result?.status),
                        ),
                        t.span({ className: "label" }, () => `${data.stepResults.length} step result(s)`),
                    ),
                    () => {
                        if (!data.stepResults.length) {
                            return null;
                        }

                        return t.div(
                            { className: "list m-b-sm" },
                            () =>
                                t.div(
                                    { className: "automation-dry-run-results" },
                                    ...data.stepResults.map((result, index) =>
                                        t.div(
                                            { rid: `${index}_${result.type}`, className: "list-item" },
                                            t.div(
                                                { className: "content block" },
                                                t.div(
                                                    { className: "flex gap-5 flex-wrap" },
                                                    t.span({ className: "txt-bold" }, `Step ${index + 1}`),
                                                    t.span({ className: "label" }, result.type || "unknown"),
                                                    t.span(
                                                        {
                                                            className: () =>
                                                                `label ${stepResultStatusClass(result.status)}`,
                                                        },
                                                        () => formatStepResultStatus(result.status),
                                                    ),
                                                ),
                                                () =>
                                                    result.error
                                                        ? t.div({ className: "txt-sm txt-danger m-t-5" }, result.error)
                                                        : null,
                                            ),
                                        )
                                    ),
                                ),
                        );
                    },
                    t.div(
                        { className: "field" },
                        t.label(null, "Raw dry-run JSON"),
                        app.components.codeBlock({
                            language: "js",
                            value: () => data.prettyResult,
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
                    disabled: () => data.isRunning,
                    onclick: () => app.modals.close(modal),
                },
                t.span({ className: "txt" }, "Close"),
            ),
            t.button(
                {
                    type: "button",
                    className: () => `btn ${data.isRunning ? "loading" : ""}`,
                    disabled: () => data.isRunning,
                    onclick: runPreview,
                },
                t.i({ className: "ri-play-line", ariaHidden: true }),
                t.span({ className: "txt" }, "Run preview"),
            ),
        ),
    );

    return modal;
}

function parseJSONObject(raw) {
    let parsed;
    try {
        parsed = JSON.parse(raw);
    } catch (_) {
        throw new Error("Input must be valid JSON.");
    }

    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
        throw new Error("Input must be a JSON object.");
    }

    return parsed;
}

function stringifyJSON(value) {
    try {
        return JSON.stringify(value || {}, null, 2);
    } catch (_) {
        return "{}";
    }
}

function runStatusClass(status) {
    switch (status) {
        case "success":
            return "success";
        case "failed":
            return "danger";
        case "running":
            return "info";
        default:
            return "";
    }
}

function stepResultStatusClass(status) {
    switch (status) {
        case "success":
        case "skipped":
            return "success";
        case "failed":
            return "danger";
        case "dry-run":
            return "info";
        default:
            return "";
    }
}

function formatRunStatus(status) {
    return status || "unknown";
}

function formatStepResultStatus(status) {
    return status || "unknown";
}
