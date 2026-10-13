export const manualInputTypeOptions = [
    { value: "text", label: "Text" },
    { value: "number", label: "Number" },
    { value: "boolean", label: "Boolean" },
    { value: "json", label: "JSON" },
];

const manualInputNamePattern = /^[a-zA-Z_][a-zA-Z0-9_]*$/;

/**
 * Converts the stored manual inputs (or already normalized editor rows)
 * into editor rows.
 */
export function normalizeManualInputRows(inputs) {
    if (!Array.isArray(inputs)) {
        return [];
    }

    return inputs.map((input) => ({
        __id: input?.__id || app.utils.randomString(),
        name: String(input?.name || ""),
        type: manualInputTypeOptions.some((option) => option.value === input?.type) ? input.type : "text",
        required: !!input?.required,
        defaultText: input?.defaultText !== undefined
            ? String(input.defaultText)
            : stringifyManualInputValue(input?.default),
        description: String(input?.description || ""),
    }));
}

/**
 * Builds the `manualInputs` API payload from the editor rows.
 * Throws an error with a user friendly message for invalid rows.
 */
export function buildManualInputsPayload(rows) {
    const seen = new Set();

    return (rows || []).map((row, index) => {
        const name = String(row.name || "").trim();
        if (!manualInputNamePattern.test(name)) {
            throw new Error(
                `Input ${
                    index + 1
                }: name must start with a letter or underscore and contain only letters, digits and underscores.`,
            );
        }
        if (seen.has(name)) {
            throw new Error(`Input "${name}" is defined more than once.`);
        }
        seen.add(name);

        const input = { name, type: row.type || "text" };
        if (row.required) {
            input.required = true;
        }

        const description = String(row.description || "").trim();
        if (description) {
            input.description = description;
        }

        if (String(row.defaultText || "").trim() !== "") {
            try {
                input.default = parseManualInputValue(input.type, row.defaultText);
            } catch (err) {
                throw new Error(`Input "${name}" default value: ${err.message}`);
            }
        }

        return input;
    });
}

/**
 * Parses a raw form value into the declared input type.
 */
export function parseManualInputValue(type, raw) {
    switch (type) {
        case "number": {
            const value = Number(String(raw).trim());
            if (String(raw).trim() === "" || !Number.isFinite(value)) {
                throw new Error("must be a number.");
            }
            return value;
        }
        case "boolean":
            if (typeof raw === "boolean") {
                return raw;
            }
            if (!["true", "false"].includes(String(raw).trim())) {
                throw new Error("must be true or false.");
            }
            return String(raw).trim() === "true";
        case "json":
            try {
                return JSON.parse(raw);
            } catch (_) {
                throw new Error("must be valid JSON.");
            }
        default:
            return String(raw);
    }
}

function stringifyManualInputValue(value) {
    if (value === undefined || value === null) {
        return "";
    }

    if (typeof value === "string") {
        return value;
    }

    try {
        return JSON.stringify(value);
    } catch (_) {
        return String(value);
    }
}

function defaultPlaceholder(type) {
    switch (type) {
        case "number":
            return "e.g. 10";
        case "boolean":
            return "true or false";
        case "json":
            return `e.g. {"key": "value"}`;
        default:
            return "Optional default";
    }
}

/**
 * Editor for the manual trigger input fields.
 *
 * Rows are mutated in place (only add/remove/reorder change the array).
 */
export function manualInputsEditor({ rows, idPrefix = "manual_input" }) {
    function addRow() {
        rows().push(...normalizeManualInputRows([{ type: "text" }]));
    }

    function removeRow(index) {
        rows().splice(index, 1);
    }

    return t.div(
        { className: "automation-manual-inputs" },
        () => {
            const list = rows();
            if (!list.length) {
                return t.div(
                    { className: "txt-sm txt-hint automation-field-desc" },
                    "No input fields. The automation can still receive arbitrary data as {{input.*}}.",
                );
            }

            return t.div(
                { className: "automation-manual-input-rows" },
                ...list.map((row, index) =>
                    t.div(
                        { rid: row.__id, className: "automation-manual-input-row" },
                        t.div(
                            { className: "automation-manual-input-row-head" },
                            t.input({
                                id: `${idPrefix}_${row.__id}_name`,
                                type: "text",
                                className: "txt-mono",
                                placeholder: "name",
                                ariaLabel: "Input name",
                                maxlength: 100,
                                value: () => row.name,
                                oninput: (e) => (row.name = e.target.value),
                            }),
                            t.button(
                                {
                                    type: "button",
                                    className: "btn sm circle transparent secondary",
                                    ariaLabel: app.attrs.tooltip("Remove input"),
                                    onclick: () => removeRow(index),
                                },
                                t.i({ className: "ti ti-x", ariaHidden: true }),
                            ),
                        ),
                        t.div(
                            { className: "automation-manual-input-row-meta" },
                            app.components.select({
                                id: `${idPrefix}_${row.__id}_type`,
                                className: "automation-manual-input-type",
                                value: () => row.type,
                                options: manualInputTypeOptions,
                                onchange: (selected) => (row.type = selected?.[0]?.value || "text"),
                            }),
                            t.div(
                                { className: "field automation-manual-input-required" },
                                t.input({
                                    id: `${idPrefix}_${row.__id}_required`,
                                    type: "checkbox",
                                    className: "sm",
                                    checked: () => row.required,
                                    onchange: (e) => (row.required = e.target.checked),
                                }),
                                t.label({ htmlFor: `${idPrefix}_${row.__id}_required` }, "Required"),
                            ),
                        ),
                        t.input({
                            id: `${idPrefix}_${row.__id}_default`,
                            type: "text",
                            ariaLabel: "Default value",
                            placeholder: () => defaultPlaceholder(row.type),
                            value: () => row.defaultText,
                            oninput: (e) => (row.defaultText = e.target.value),
                        }),
                    )
                ),
            );
        },
        t.button(
            {
                type: "button",
                className: "btn sm secondary transparent automation-manual-input-add",
                onclick: addRow,
            },
            t.i({ className: "ti ti-plus", ariaHidden: true }),
            t.span({ className: "txt" }, "Add input field"),
        ),
    );
}

/**
 * Opens a modal for running a manual automation with input data.
 *
 * @param {Object} automation
 * @param {Object} [options]
 * @param {Function} [options.onrun] Called after the run was started/completed.
 */
export function openAutomationRunModal(automation, options = {}) {
    if (!automation?.id) {
        return;
    }

    const modal = automationRunModal(automation, options);
    document.body.appendChild(modal);
    app.modals.open(modal);
}

function automationRunModal(automation, options) {
    const formId = "automation_run_" + app.utils.randomString();
    const inputs = Array.isArray(automation.manualInputs) ? automation.manualInputs : [];

    const initialValues = {};
    for (const input of inputs) {
        initialValues[input.name] = input.type === "boolean"
            ? input.default === true
            : stringifyManualInputValue(input.default);
    }

    const data = store({
        isRunning: false,
        wait: true,
        values: initialValues,
        jsonText: "{}",
        fieldErrors: {},
        result: null,
        get prettyOutput() {
            const output = data.result?.output ?? null;
            if (typeof output === "string") {
                return output;
            }
            try {
                return JSON.stringify(output, null, 2);
            } catch (_) {
                return String(output);
            }
        },
    });

    let modal;

    function collectInput() {
        if (!inputs.length) {
            let parsed;
            try {
                parsed = JSON.parse(data.jsonText || "{}");
            } catch (_) {
                throw new Error("Input must be valid JSON.");
            }
            if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
                throw new Error("Input must be a JSON object.");
            }
            return parsed;
        }

        const result = {};
        const errors = {};
        for (const input of inputs) {
            const raw = data.values[input.name];
            if (raw === "" || raw === undefined || raw === null) {
                continue; // let the server apply defaults/required checks
            }
            try {
                result[input.name] = parseManualInputValue(input.type, raw);
            } catch (err) {
                errors[input.name] = `Must ${err.message.replace(/^must /, "be ").replace(/\.$/, "")}.`;
            }
        }

        data.fieldErrors = errors;
        if (Object.keys(errors).length) {
            throw new Error("Please fix the highlighted fields.");
        }

        return result;
    }

    async function run() {
        if (data.isRunning) {
            return;
        }

        let input;
        try {
            input = collectInput();
        } catch (err) {
            app.toasts.error(err.message);
            return;
        }

        data.isRunning = true;
        data.result = null;
        data.fieldErrors = {};

        try {
            const result = await app.pb.send(`/api/automations/${automation.id}/run`, {
                method: "POST",
                body: { input, wait: data.wait },
                requestKey: null,
            });

            options.onrun?.(result);

            if (data.wait) {
                data.result = result;
            } else {
                app.toasts.success(`Started "${automation.name}".`);
                app.modals.close(modal, true);
            }
        } catch (err) {
            if (!err?.isAbort) {
                const errData = err?.response?.data || {};
                const fieldErrors = {};
                for (const input of inputs) {
                    if (errData[input.name]?.message) {
                        fieldErrors[input.name] = errData[input.name].message;
                    }
                }
                data.fieldErrors = fieldErrors;

                if (!Object.keys(fieldErrors).length) {
                    app.checkApiError(err);
                }
            }
        }

        data.isRunning = false;
    }

    function renderInputField(input) {
        const id = `${formId}_${input.name}`;
        const label = t.label({ htmlFor: id }, input.label || input.name, input.required ? " *" : "");

        let control;
        switch (input.type) {
            case "boolean":
                return t.div(
                    { className: "col-lg-12" },
                    t.div(
                        { className: "field" },
                        t.input({
                            id,
                            type: "checkbox",
                            className: "switch",
                            checked: () => !!data.values[input.name],
                            onchange: (e) => (data.values[input.name] = e.target.checked),
                        }),
                        label,
                    ),
                    fieldMeta(input),
                );
            case "json":
                control = t.textarea({
                    id,
                    rows: 4,
                    spellcheck: false,
                    className: "txt-mono",
                    placeholder: `{"key": "value"}`,
                    value: () => data.values[input.name],
                    oninput: (e) => (data.values[input.name] = e.target.value),
                });
                break;
            case "number":
                control = t.input({
                    id,
                    type: "number",
                    step: "any",
                    value: () => data.values[input.name],
                    oninput: (e) => (data.values[input.name] = e.target.value),
                });
                break;
            default:
                control = t.input({
                    id,
                    type: "text",
                    value: () => data.values[input.name],
                    oninput: (e) => (data.values[input.name] = e.target.value),
                });
        }

        return t.div(
            { className: "col-lg-12" },
            t.div({ className: "field" }, label, control),
            fieldMeta(input),
        );
    }

    function fieldMeta(input) {
        return [
            () => input.description ? t.div({ className: "field-help" }, input.description) : null,
            () =>
                data.fieldErrors[input.name]
                    ? t.div({ className: "field-error txt-danger m-t-sm" }, data.fieldErrors[input.name])
                    : null,
        ];
    }

    modal = t.div(
        {
            pbEvent: "automationRunModal",
            className: "modal popup automation-run-modal",
            onafterclose: (el) => el?.remove(),
        },
        t.header(
            { className: "modal-header" },
            t.h5(null, `Run "${automation.name || automation.id}"`),
        ),
        t.form(
            {
                id: formId,
                className: "modal-content",
                onsubmit: (e) => {
                    e.preventDefault();
                    run();
                },
            },
            t.div(
                { className: "grid" },
                ...(inputs.length
                    ? inputs.map(renderInputField)
                    : [
                        t.div(
                            { className: "col-lg-12" },
                            t.div(
                                { className: "field" },
                                t.label({ htmlFor: formId + "_json" }, "Input JSON"),
                                t.textarea({
                                    id: formId + "_json",
                                    rows: 6,
                                    spellcheck: false,
                                    className: "txt-mono",
                                    value: () => data.jsonText,
                                    oninput: (e) => (data.jsonText = e.target.value),
                                }),
                            ),
                            t.div(
                                { className: "field-help" },
                                "Available in the automation as {{input.*}}. Declare input fields on the manual trigger to get a form here.",
                            ),
                        ),
                    ]),
                t.div(
                    { className: "col-lg-12" },
                    t.div(
                        { className: "field" },
                        t.input({
                            id: formId + "_wait",
                            type: "checkbox",
                            className: "switch",
                            checked: () => data.wait,
                            onchange: (e) => (data.wait = e.target.checked),
                        }),
                        t.label({ htmlFor: formId + "_wait" }, "Wait for the result"),
                    ),
                ),
                t.div(
                    { className: "col-lg-12", hidden: () => !data.result },
                    t.div(
                        { className: "flex gap-5 flex-wrap m-b-sm" },
                        t.span(
                            {
                                className: () =>
                                    `label ${
                                        data.result?.status === "success"
                                            ? "success"
                                            : data.result?.status === "failed"
                                            ? "danger"
                                            : "info"
                                    }`,
                            },
                            () => data.result?.status || "",
                        ),
                        t.span({ className: "label txt-mono" }, () => `run ${data.result?.runId || ""}`),
                    ),
                    () =>
                        data.result?.error
                            ? t.div({ className: "txt-sm txt-danger m-b-sm" }, data.result.error)
                            : null,
                    t.div(
                        { className: "field" },
                        t.label(null, "Output"),
                        app.components.codeBlock({
                            language: "js",
                            value: () => data.prettyOutput,
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
                    type: "submit",
                    "html-form": formId,
                    className: () => `btn ${data.isRunning ? "loading" : ""}`,
                    disabled: () => data.isRunning,
                },
                t.i({ className: "ri-play-line", ariaHidden: true }),
                t.span({ className: "txt" }, () => data.result ? "Run again" : "Run"),
            ),
        ),
    );

    return modal;
}
