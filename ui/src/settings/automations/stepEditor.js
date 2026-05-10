import { conditionStepForm } from "./conditionStepForm";
import { httpStepForm } from "./httpStepForm";
import { mailStepForm } from "./mailStepForm";
import { recordStepForm } from "./recordStepForm";
import { responseStepForm } from "./responseStepForm";

const stepTypeOptions = [
    { value: "condition", label: "Condition" },
    { value: "http", label: "HTTP request" },
    { value: "mail.send", label: "Send mail" },
    { value: "record.create", label: "Create record" },
    { value: "record.update", label: "Update record" },
    { value: "record.delete", label: "Delete record" },
    { value: "response", label: "Webhook response", triggerTypes: ["webhook"] },
];

export function stepEditor(propsArg = {}) {
    const props = store({
        steps: [],
        errors: null,
        triggerType: "",
        triggerCollectionRef: "",
        onchange: function(steps) {},
    });

    const watchers = app.utils.extendStore(props, propsArg);

    const data = store({
        expandedById: {},
    });

    function setSteps(steps) {
        props.onchange?.(steps);
    }

    function addStep(type) {
        const nextStep = createEditorStep(type);
        data.expandedById[nextStep.__id] = true;
        setSteps([...(props.steps || []), nextStep]);
    }

    function removeStep(stepId) {
        const nextSteps = (props.steps || []).filter((step) => step.__id !== stepId);
        delete data.expandedById[stepId];
        setSteps(nextSteps);
    }

    function changeStepType(step, nextType) {
        resetStep(step, createEditorStep(nextType, { __id: step.__id }));
        data.expandedById[step.__id] = true;
    }

    function toggleExpanded(stepId) {
        data.expandedById[stepId] = !isExpanded(stepId);
    }

    function isExpanded(stepId) {
        return data.expandedById[stepId] !== false;
    }

    return t.div(
        {
            pbEvent: "automationStepEditor",
            className: "automation-step-editor",
            onunmount: () => {
                watchers.forEach((w) => w?.unwatch());
            },
        },
        () => {
            const stepsError = extractErrorMessage(resolveStepListError(props.errors));
            if (!stepsError) {
                return null;
            }

            return t.div(
                { className: "alert danger m-b-sm" },
                t.div({ className: "content" }, stepsError),
            );
        },
        app.components.sortable({
            className: "list automation-steps-list",
            handle: ".sort-handle",
            data: () => props.steps || [],
            onchange: (sortedSteps) => setSteps(sortedSteps),
            before: () => {
                if (props.steps?.length) {
                    return null;
                }

                return t.div(
                    { className: "list-item" },
                    t.div(
                        { className: "content block txt-hint" },
                        "No steps added yet. Add a condition, HTTP request, mail step, or record action below.",
                    ),
                );
            },
            dataItem: (step, index) => {
                const stepError = resolveStepError(props.errors, index);

                return t.div(
                    { rid: step.__id, className: "list-item automation-step-item" },
                    t.div(
                        { className: "content block" },
                        t.div(
                            { className: "flex gap-10 flex-wrap m-b-sm" },
                            t.span(
                                {
                                    className: "label handle sort-handle",
                                    title: "Reorder step",
                                },
                                t.i({ className: "ri-draggable", ariaHidden: true }),
                                t.span({ className: "txt" }, () => `Step ${index + 1}`),
                            ),
                            t.span({ className: "txt-bold" }, () => summarizeStep(step)),
                            t.div({ className: "m-l-auto" }),
                            t.button(
                                {
                                    type: "button",
                                    className: "btn sm secondary transparent circle",
                                    ariaLabel: app.attrs.tooltip(isExpanded(step.__id) ? "Collapse" : "Expand"),
                                    onclick: () => toggleExpanded(step.__id),
                                },
                                t.i({
                                    className: () =>
                                        isExpanded(step.__id) ? "ri-arrow-up-s-line" : "ri-arrow-down-s-line",
                                    ariaHidden: true,
                                }),
                            ),
                            t.button(
                                {
                                    type: "button",
                                    className: "btn sm secondary transparent circle",
                                    ariaLabel: app.attrs.tooltip("Remove"),
                                    onclick: () => removeStep(step.__id),
                                },
                                t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
                            ),
                        ),
                        t.div(
                            { className: "grid" },
                            t.div(
                                { className: "col-lg-4" },
                                t.div(
                                    { className: "field" },
                                    t.label({ htmlFor: `${step.__id}_type` }, "Step type"),
                                    app.components.select({
                                        id: `${step.__id}_type`,
                                        value: () => step.type,
                                        options: () => stepTypeSelectOptions(props.triggerType, step.type),
                                        onchange: (selected) => {
                                            const nextType = selected?.[0]?.value || "condition";
                                            if (nextType !== step.type) {
                                                changeStepType(step, nextType);
                                            }
                                        },
                                    }),
                                ),
                            ),
                        ),
                        app.components.slide(
                            () => isExpanded(step.__id),
                            t.div(
                                { className: "m-t-sm" },
                                () =>
                                    renderStepForm(step, stepError, {
                                        triggerType: props.triggerType,
                                        triggerCollectionRef: props.triggerCollectionRef,
                                    }),
                            ),
                        ),
                        () => {
                            const message = extractErrorMessage(stepError);
                            if (!message) {
                                return null;
                            }

                            return t.div(
                                { className: "field-error txt-danger m-t-sm" },
                                message,
                            );
                        },
                    ),
                );
            },
            after: () => {
                return t.div(
                    { className: "list-item block" },
                    t.div(
                        { className: "flex gap-5 flex-wrap" },
                        () =>
                            stepTypeAddOptions(props.triggerType).map((option) => {
                                return t.button(
                                    {
                                        rid: option.value,
                                        type: "button",
                                        className: "btn sm secondary transparent",
                                        onclick: () => addStep(option.value),
                                    },
                                    t.i({ className: "ri-add-line", ariaHidden: true }),
                                    t.span({ className: "txt" }, option.label),
                                );
                            }),
                    ),
                );
            },
        }),
    );
}

export function normalizeAutomationEditorSteps(steps) {
    if (typeof steps === "string" && steps.trim()) {
        try {
            steps = JSON.parse(steps);
        } catch (_) {
            steps = [];
        }
    }

    if (!Array.isArray(steps)) {
        return [];
    }

    return steps.map((step) => createEditorStep(step?.type, step));
}

export function buildAutomationStepsPayload(steps) {
    return (steps || []).map((step, index) => buildStepPayload(step, index));
}

function renderStepForm(step, error, context = {}) {
    switch (step.type) {
        case "condition":
            return conditionStepForm({ step, error });
        case "http":
            return httpStepForm({ step, error });
        case "mail.send":
            return mailStepForm({
                step,
                error,
                triggerType: () => context.triggerType,
                triggerCollectionRef: () => context.triggerCollectionRef,
            });
        case "record.create":
        case "record.update":
        case "record.delete":
            return recordStepForm({ step, error });
        case "response":
            return responseStepForm({ step, error });
        default:
            return t.div({ className: "txt-sm txt-danger" }, `Unsupported step type "${step.type}".`);
    }
}

function stepTypeAddOptions(triggerType) {
    return stepTypeOptions.filter((option) => !option.triggerTypes || option.triggerTypes.includes(triggerType));
}

function stepTypeSelectOptions(triggerType, selectedType) {
    const options = stepTypeAddOptions(triggerType);
    if (options.find((option) => option.value === selectedType)) {
        return options;
    }

    const selected = stepTypeOptions.find((option) => option.value === selectedType);
    return selected ? [selected, ...options] : options;
}

function createEditorStep(type, rawStep = {}) {
    const base = {
        __id: rawStep.__id || app.utils.randomString(),
        type: type || rawStep?.type || "condition",
    };

    switch (base.type) {
        case "condition":
            return {
                ...base,
                path: toString(rawStep.path || rawStep.field),
                op: toString(rawStep.op) || "exists",
                valueText: stringifyLooseValue(rawStep.value),
            };
        case "http":
            return {
                ...base,
                method: toString(rawStep.method) || "GET",
                url: toString(rawStep.url),
                headersText: stringifyJSONObject(rawStep.headers, "{}"),
                bodyText: stringifyLooseValue(rawStep.body),
                timeoutText: rawStep.timeout === undefined || rawStep.timeout === null ? "" : String(rawStep.timeout),
            };
        case "mail.send":
            return {
                ...base,
                toText: stringifyStringArray(rawStep.to),
                ccText: stringifyStringArray(rawStep.cc),
                bccText: stringifyStringArray(rawStep.bcc),
                subject: toString(rawStep.subject),
                text: toString(rawStep.text),
                html: toString(rawStep.html),
                attachments: normalizeStringArray(rawStep.attachments),
            };
        case "record.create":
            return {
                ...base,
                collection: toString(rawStep.collection),
                dataText: stringifyJSONObject(rawStep.data, "{}"),
            };
        case "record.update":
            return {
                ...base,
                collection: toString(rawStep.collection),
                id: toString(rawStep.id),
                filter: toString(rawStep.filter),
                dataText: stringifyJSONObject(rawStep.data, "{}"),
            };
        case "record.delete":
            return {
                ...base,
                collection: toString(rawStep.collection),
                id: toString(rawStep.id),
                filter: toString(rawStep.filter),
            };
        case "response":
            return {
                ...base,
                statusCodeText: rawStep.statusCode === undefined || rawStep.statusCode === null
                    ? "200"
                    : String(rawStep.statusCode),
                headersText: stringifyJSONObject(rawStep.headers, "{}"),
                bodyText: stringifyLooseValue(rawStep.body),
            };
        default:
            return createEditorStep("condition", { __id: base.__id });
    }
}

function buildStepPayload(step, index) {
    switch (step.type) {
        case "condition":
            return buildConditionPayload(step, index);
        case "http":
            return buildHTTPPayload(step, index);
        case "mail.send":
            return buildMailPayload(step, index);
        case "record.create":
            return buildRecordCreatePayload(step, index);
        case "record.update":
            return buildRecordUpdatePayload(step, index);
        case "record.delete":
            return buildRecordDeletePayload(step, index);
        case "response":
            return buildResponsePayload(step, index);
        default:
            throw new Error(`Step ${index + 1}: unsupported step type "${step.type}".`);
    }
}

function buildConditionPayload(step, index) {
    const path = step.path.trim();
    if (!path) {
        throw new Error(`Step ${index + 1}: condition path is required.`);
    }

    const payload = {
        type: "condition",
        path,
        op: step.op || "exists",
    };

    if (payload.op !== "exists") {
        const value = parseLooseValue(step.valueText);
        if (payload.op === "in" && !Array.isArray(value)) {
            throw new Error(
                `Step ${index + 1}: "in" condition value must be a JSON array or a template-rendered array.`,
            );
        }

        payload.value = value;
    }

    return payload;
}

function buildHTTPPayload(step, index) {
    const url = step.url.trim();
    if (!url) {
        throw new Error(`Step ${index + 1}: HTTP url is required.`);
    }

    const payload = {
        type: "http",
        method: (step.method || "GET").trim().toUpperCase(),
        url,
    };

    const headersText = step.headersText.trim();
    if (headersText) {
        payload.headers = parseJSONObject(headersText, `Step ${index + 1}: HTTP headers`);
    }

    const bodyText = step.bodyText.trim();
    if (bodyText) {
        payload.body = parseLooseValue(bodyText);
    }

    const timeoutText = step.timeoutText.trim();
    if (timeoutText) {
        const timeout = Number(timeoutText);
        if (!Number.isFinite(timeout) || timeout <= 0) {
            throw new Error(`Step ${index + 1}: HTTP timeout must be greater than zero.`);
        }

        payload.timeout = timeout;
    }

    return payload;
}

function buildRecordCreatePayload(step, index) {
    const collection = step.collection.trim();
    if (!collection) {
        throw new Error(`Step ${index + 1}: record collection is required.`);
    }

    return {
        type: "record.create",
        collection,
        data: parseJSONObject(step.dataText, `Step ${index + 1}: record data`),
    };
}

function buildMailPayload(step, index) {
    const to = parseStringList(step.toText);
    if (!to.length) {
        throw new Error(`Step ${index + 1}: at least one mail recipient is required.`);
    }

    const subject = step.subject.trim();
    if (!subject) {
        throw new Error(`Step ${index + 1}: mail subject is required.`);
    }

    const text = step.text.trim();
    const html = step.html.trim();
    if (!text && !html) {
        throw new Error(`Step ${index + 1}: mail step requires text or HTML content.`);
    }

    const payload = {
        type: "mail.send",
        to,
        subject,
    };

    const cc = parseStringList(step.ccText);
    if (cc.length) {
        payload.cc = cc;
    }

    const bcc = parseStringList(step.bccText);
    if (bcc.length) {
        payload.bcc = bcc;
    }

    if (text) {
        payload.text = text;
    }

    if (html) {
        payload.html = html;
    }

    const attachments = normalizeStringArray(step.attachments);
    if (attachments.length) {
        payload.attachments = attachments;
    }

    return payload;
}

function buildRecordUpdatePayload(step, index) {
    const payload = buildRecordCreatePayload({
        ...step,
        type: "record.create",
    }, index);

    payload.type = "record.update";

    const id = step.id.trim();
    const filter = step.filter.trim();
    if (!id && !filter) {
        throw new Error(`Step ${index + 1}: record update requires either id or filter.`);
    }

    if (id) {
        payload.id = id;
    }
    if (filter) {
        payload.filter = filter;
    }

    return payload;
}

function buildRecordDeletePayload(step, index) {
    const collection = step.collection.trim();
    if (!collection) {
        throw new Error(`Step ${index + 1}: record collection is required.`);
    }

    const id = step.id.trim();
    const filter = step.filter.trim();
    if (!id && !filter) {
        throw new Error(`Step ${index + 1}: record delete requires either id or filter.`);
    }

    const payload = {
        type: "record.delete",
        collection,
    };

    if (id) {
        payload.id = id;
    }
    if (filter) {
        payload.filter = filter;
    }

    return payload;
}

function buildResponsePayload(step, index) {
    const payload = {
        type: "response",
    };

    const statusCodeText = step.statusCodeText.trim();
    if (statusCodeText) {
        const statusCode = Number(statusCodeText);
        if (!Number.isInteger(statusCode) || statusCode < 100 || statusCode > 599) {
            throw new Error(`Step ${index + 1}: response status code must be an integer between 100 and 599.`);
        }

        payload.statusCode = statusCode;
    }

    const headersText = step.headersText.trim();
    if (headersText) {
        payload.headers = parseJSONObject(headersText, `Step ${index + 1}: response headers`);
    }

    const bodyText = step.bodyText.trim();
    if (bodyText) {
        payload.body = parseLooseValue(bodyText);
    }

    return payload;
}

function summarizeStep(step) {
    switch (step.type) {
        case "condition":
            return `${step.path || "Condition path"} • ${step.op || "exists"}`;
        case "http":
            return `${(step.method || "GET").toUpperCase()} ${step.url || "HTTP request"}`;
        case "mail.send":
            return `Send mail to ${firstStringListValue(step.toText) || "recipient"}`;
        case "record.create":
            return `Create record in ${step.collection || "collection"}`;
        case "record.update":
            return `Update record in ${step.collection || "collection"}`;
        case "record.delete":
            return `Delete record in ${step.collection || "collection"}`;
        case "response":
            return `Return webhook response ${step.statusCodeText || "200"}`;
        default:
            return step.type || "Step";
    }
}

function resolveStepListError(errors) {
    if (!errors || typeof errors !== "object") {
        return null;
    }

    const keys = Object.keys(errors).filter((key) => !/^\d+$/.test(key));
    if (!keys.length) {
        return null;
    }

    return errors;
}

function resolveStepError(errors, index) {
    if (!errors || typeof errors !== "object") {
        return null;
    }

    return errors[index] || errors[String(index)] || null;
}

function extractErrorMessage(value) {
    if (!value) {
        return "";
    }

    if (typeof value?.message === "string") {
        return value.message;
    }

    if (Array.isArray(value)) {
        return value.map((entry) => extractErrorMessage(entry)).filter(Boolean).join("\n");
    }

    if (typeof value === "object") {
        const nested = Object.values(value).map((entry) => extractErrorMessage(entry)).filter(Boolean);
        if (nested.length) {
            return nested[0];
        }
    }

    return String(value || "");
}

function resetStep(target, source) {
    for (const key of Object.keys(target)) {
        delete target[key];
    }

    Object.assign(target, source);
}

function parseJSONObject(raw, label) {
    let parsed;
    try {
        parsed = JSON.parse(raw);
    } catch (_) {
        throw new Error(`${label} must be valid JSON.`);
    }

    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
        throw new Error(`${label} must be a JSON object.`);
    }

    return parsed;
}

function parseLooseValue(raw) {
    const trimmed = raw.trim();
    if (!trimmed) {
        return "";
    }

    try {
        return JSON.parse(trimmed);
    } catch (_) {
        return raw;
    }
}

function stringifyJSONObject(value, fallback = "{}") {
    if (value === undefined || value === null || value === "") {
        return fallback;
    }

    if (typeof value === "string") {
        try {
            value = JSON.parse(value);
        } catch (_) {
            return value;
        }
    }

    try {
        return JSON.stringify(value, null, 2);
    } catch (_) {
        return fallback;
    }
}

function stringifyLooseValue(value) {
    if (value === undefined || value === null) {
        return "";
    }

    if (typeof value === "string") {
        return value;
    }

    try {
        return JSON.stringify(value, null, 2);
    } catch (_) {
        return String(value);
    }
}

function stringifyStringArray(value) {
    return normalizeStringArray(value).join("\n");
}

function normalizeStringArray(value) {
    if (!Array.isArray(value)) {
        return [];
    }

    return value
        .map((item) => toString(item).trim())
        .filter(Boolean);
}

function parseStringList(raw) {
    return raw
        .split(/\r?\n|,/)
        .map((item) => item.trim())
        .filter(Boolean);
}

function firstStringListValue(raw) {
    return parseStringList(raw || "")[0] || "";
}

function toString(value) {
    if (value === undefined || value === null) {
        return "";
    }

    return String(value);
}
