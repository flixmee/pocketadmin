import { openAutomationDryRunModal } from "./automationDryRunModal";
import { openAutomationRunsModal } from "./automationRunsList";
import { buildAutomationStepsPayload, normalizeAutomationEditorSteps, stepEditor } from "./stepEditor";

const automationTriggerOptions = [
    { value: "manual", label: "Manual" },
    { value: "webhook", label: "Webhook" },
    { value: "schedule.cron", label: "Scheduled cron" },
    { value: "record.create", label: "Record create" },
    { value: "record.update", label: "Record update" },
    { value: "record.delete", label: "Record delete" },
    { value: "i18n.translation_missing", label: "Translation missing" },
    { value: "i18n.locale_published", label: "Locale published" },
    { value: "i18n.translation_updated", label: "Translation updated" },
    { value: "i18n.ai_translation_finished", label: "AI translation finished" },
];

export function openAutomationUpsertModal(automation = null, settings = {
    onsave: null,
}) {
    const modal = automationUpsertModal(automation, settings);
    if (!modal) {
        return;
    }

    document.body.appendChild(modal);
    app.modals.open(modal);
}

function automationUpsertModal(automation, settings) {
    let modal;

    const formId = "automation_upsert_" + app.utils.randomString();
    const isNew = !automation?.id;
    const initialForm = normalizeAutomationForm(automation);

    const data = store({
        isSaving: false,
        form: cloneAutomationForm(initialForm),
        initialSerialized: JSON.stringify(initialForm),
        get title() {
            return isNew ? "Create automation" : "Edit automation";
        },
        get submitLabel() {
            return isNew ? "Create automation" : "Save changes";
        },
        get hasChanges() {
            return JSON.stringify(normalizeAutomationForm(data.form)) !== data.initialSerialized;
        },
        get isRecordTrigger() {
            return isRecordAutomationTrigger(data.form.triggerType);
        },
        get isI18nTrigger() {
            return isI18nAutomationTrigger(data.form.triggerType);
        },
        get isCronTrigger() {
            return data.form.triggerType === "schedule.cron";
        },
        get isWebhookTrigger() {
            return data.form.triggerType === "webhook";
        },
        get canSave() {
            return !data.isSaving && !!data.form.name.trim() && data.form.steps.length > 0 && data.hasChanges;
        },
    });

    function collectionOptions(selectedValue = "") {
        const options = (app.store.collections || [])
            .filter((collection) => collection?.type === "base" || collection?.type === "auth")
            .map((collection) => ({
                value: collection.id,
                label: `${collection.name} (${collection.type})`,
            }));

        if (selectedValue && !options.find((option) => option.value === selectedValue)) {
            options.unshift({
                value: selectedValue,
                label: selectedValue,
            });
        }

        return options;
    }

    function setTriggerType(triggerType) {
        data.form.triggerType = triggerType;

        if (!isRecordAutomationTrigger(triggerType) && !isI18nAutomationTrigger(triggerType)) {
            data.form.collectionRef = "";
        }
        if (triggerType !== "schedule.cron") {
            data.form.cronExpr = "";
        }
    }

    async function save() {
        if (data.isSaving) {
            return;
        }

        let payload;
        try {
            payload = buildAutomationPayload(data.form);
        } catch (err) {
            app.toasts.error(err.message || "Invalid automation configuration.");
            return;
        }

        data.isSaving = true;
        app.store.errors = null;

        try {
            const saved = automation?.id
                ? await app.pb.send(`/api/automations/${automation.id}`, {
                    method: "PATCH",
                    body: payload,
                })
                : await app.pb.send("/api/automations", {
                    method: "POST",
                    body: payload,
                });

            settings.onsave?.(saved, isNew);
            app.toasts.success(isNew ? "Automation created." : "Automation updated.");
            app.modals.close(modal, true);
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
                data.isSaving = false;
            }
            return;
        }

        data.isSaving = false;
    }

    function openRunsModal() {
        if (!automation?.id || data.isSaving) {
            return;
        }

        openAutomationRunsModal(automation);
    }

    function openDryRunModal() {
        if (!automation?.id || data.isSaving) {
            return;
        }

        openAutomationDryRunModal(automation);
    }

    modal = t.div(
        {
            pbEvent: "automationUpsertModal",
            className: "modal popup automation-upsert-modal",
            onafterclose: (el) => {
                if (app.store.errors) {
                    app.store.errors = null;
                }
                el?.remove();
            },
        },
        t.header({ className: "modal-header" }, t.h5({ className: "m-auto" }, () => data.title)),
        t.form(
            {
                id: formId,
                className: "modal-content",
                inert: () => data.isSaving,
                onsubmit: (e) => {
                    e.preventDefault();
                    save();
                },
            },
            t.div(
                { className: "grid" },
                t.div(
                    { className: "col-md-8" },
                    t.div(
                        { className: "field" },
                        t.label({ htmlFor: formId + "_name" }, "Name"),
                        t.input({
                            id: formId + "_name",
                            name: "name",
                            type: "text",
                            required: true,
                            maxlength: 255,
                            value: () => data.form.name,
                            oninput: (e) => (data.form.name = e.target.value),
                        }),
                    ),
                    () => fieldError(app.store.errors?.name),
                ),
                t.div(
                    { className: "col-md-4" },
                    t.div(
                        { className: "field m-t-lg" },
                        t.input({
                            id: formId + "_active",
                            name: "active",
                            type: "checkbox",
                            className: "switch",
                            checked: () => data.form.active,
                            onchange: (e) => (data.form.active = e.target.checked),
                        }),
                        t.label({ htmlFor: formId + "_active" }, t.span({ className: "txt" }, "Active")),
                    ),
                ),
                t.div(
                    { className: "col-md-6" },
                    t.div(
                        { className: "field" },
                        t.label({ htmlFor: formId + "_trigger" }, "Trigger"),
                        app.components.select({
                            id: formId + "_trigger",
                            name: "triggerType",
                            value: () => data.form.triggerType,
                            options: automationTriggerOptions,
                            onchange: (selected) => setTriggerType(selected?.[0]?.value || "manual"),
                        }),
                    ),
                    () => fieldError(app.store.errors?.triggerType),
                ),
                t.div(
                    {
                        className: "col-md-6",
                        hidden: () => !data.isRecordTrigger && !data.isI18nTrigger,
                    },
                    t.div(
                        { className: "field" },
                        t.label({ htmlFor: formId + "_collectionRef" }, "Target collection"),
                        app.components.select({
                            id: formId + "_collectionRef",
                            name: "collectionRef",
                            value: () => data.form.collectionRef,
                            options: () => collectionOptions(data.form.collectionRef),
                            placeholder: "- Select collection -",
                            onchange: (selected) => {
                                data.form.collectionRef = selected?.[0]?.value || "";
                            },
                        }),
                    ),
                    () => fieldError(app.store.errors?.collectionRef),
                ),
                t.div(
                    {
                        className: "col-md-6",
                        hidden: () => !data.isCronTrigger,
                    },
                    t.div(
                        { className: "field" },
                        t.label({ htmlFor: formId + "_cronExpr" }, "Cron expression"),
                        t.input({
                            id: formId + "_cronExpr",
                            name: "cronExpr",
                            type: "text",
                            placeholder: "0 * * * *",
                            value: () => data.form.cronExpr,
                            oninput: (e) => (data.form.cronExpr = e.target.value),
                        }),
                    ),
                    t.div({ className: "field-help" }, "Use standard cron syntax for scheduled automations."),
                    () => fieldError(app.store.errors?.cronExpr),
                ),
                t.div(
                    {
                        className: "col-lg-12",
                        hidden: () => !data.isWebhookTrigger,
                    },
                    t.div(
                        { className: "field" },
                        t.label(null, "Webhook endpoint"),
                        () => {
                            if (!automation?.id) {
                                return t.div(
                                    { className: "txt-sm txt-hint" },
                                    "Save the automation first to generate its stable webhook endpoint.",
                                );
                            }

                            return t.div(
                                { className: "flex gap-10 flex-wrap p-10" },
                                t.code(null, webhookURL(automation.id)),
                                app.components.copyButton(() => webhookURL(automation.id)),
                            );
                        },
                    ),
                    t.div(
                        { className: "field-help" },
                        "Send a POST request to this endpoint. Templates can read incoming values from ",
                        t.code(null, "{{request.method}}"),
                        ", ",
                        t.code(null, "{{request.headers.*}}"),
                        ", ",
                        t.code(null, "{{request.query.*}}"),
                        ", and ",
                        t.code(null, "{{request.body.*}}"),
                        ". Add a webhook response step to return custom status, headers, or body.",
                    ),
                ),
                t.div(
                    { className: "col-lg-12" },
                    t.div(
                        { className: "field" },
                        t.label({ htmlFor: formId + "_notes" }, "Notes"),
                        t.textarea({
                            id: formId + "_notes",
                            name: "notes",
                            rows: 3,
                            placeholder: "Optional internal notes for operators.",
                            value: () => data.form.notes,
                            oninput: (e) => (data.form.notes = e.target.value),
                        }),
                    ),
                    () => fieldError(app.store.errors?.notes),
                ),
                t.div(
                    { className: "col-lg-12" },
                    t.div(
                        { className: "m-b-sm" },
                        t.div({ className: "txt-bold" }, "Workflow steps"),
                        t.div(
                            { className: "txt-sm txt-hint" },
                            "Templates support ",
                            t.code(null, "{{trigger.*}}"),
                            ", ",
                            t.code(null, "{{request.*}}"),
                            ", ",
                            t.code(null, "{{record.*}}"),
                            ", ",
                            t.code(null, "{{recordOriginal.*}}"),
                            ", ",
                            t.code(null, "{{automation.*}}"),
                            ", ",
                            t.code(null, "{{run.*}}"),
                            ", ",
                            t.code(null, "{{steps[0].output.*}}"),
                            ", and ",
                            t.code(null, "{{prevStep.output.*}}"),
                            ".",
                        ),
                    ),
                    stepEditor({
                        steps: () => data.form.steps,
                        errors: () => app.store.errors?.steps,
                        triggerType: () => data.form.triggerType,
                        triggerCollectionRef: () => data.form.collectionRef,
                        onchange: (steps) => {
                            data.form = {
                                ...data.form,
                                steps,
                            };
                        },
                    }),
                ),
            ),
        ),
        t.footer(
            { className: "modal-footer" },
            t.button(
                {
                    type: "button",
                    className: "btn transparent m-r-auto",
                    disabled: () => data.isSaving,
                    onclick: () => app.modals.close(modal),
                },
                t.span({ className: "txt" }, "Close"),
            ),
            () => {
                if (isNew) {
                    return null;
                }

                return t.button(
                    {
                        type: "button",
                        className: "btn transparent",
                        disabled: () => data.isSaving,
                        onclick: openDryRunModal,
                    },
                    t.i({ className: "ri-play-circle-line", ariaHidden: true }),
                    t.span({ className: "txt" }, "Dry-run"),
                );
            },
            () => {
                if (isNew) {
                    return null;
                }

                return t.button(
                    {
                        type: "button",
                        className: "btn transparent",
                        disabled: () => data.isSaving,
                        onclick: openRunsModal,
                    },
                    t.i({ className: "ri-history-line", ariaHidden: true }),
                    t.span({ className: "txt" }, "Recent runs"),
                );
            },
            () => {
                const rawErrors = JSON.stringify(app.store.errors);
                if (rawErrors == "" || rawErrors == "null" || rawErrors == "{}" || rawErrors == "[]") {
                    return;
                }

                return t.i({
                    className: "ri-alert-line txt-danger",
                    ariaDescription: app.attrs.tooltip(() => "Raw error:\n" + rawErrors),
                });
            },
            t.button(
                {
                    type: "submit",
                    "html-form": formId,
                    className: () => `btn ${data.isSaving ? "loading" : ""}`,
                    disabled: () => !data.canSave,
                },
                t.span({ className: "txt" }, () => data.submitLabel),
            ),
        ),
    );

    return modal;
}

function normalizeAutomationForm(automation = null) {
    return {
        name: automation?.name || "",
        active: !!automation?.active,
        triggerType: automation?.triggerType || "manual",
        collectionRef: automation?.collectionRef || "",
        cronExpr: automation?.cronExpr || "",
        notes: automation?.notes || "",
        steps: normalizeAutomationEditorSteps(automation?.steps),
    };
}

function cloneAutomationForm(form) {
    return {
        ...form,
        steps: normalizeAutomationEditorSteps(form?.steps),
    };
}

function buildAutomationPayload(form) {
    return {
        name: form.name.trim(),
        active: !!form.active,
        triggerType: form.triggerType,
        collectionRef: isRecordAutomationTrigger(form.triggerType) || isI18nAutomationTrigger(form.triggerType)
            ? (form.collectionRef || "")
            : "",
        cronExpr: form.triggerType === "schedule.cron" ? form.cronExpr.trim() : "",
        notes: form.notes.trim(),
        steps: buildAutomationStepsPayload(form.steps),
    };
}

function webhookURL(automationId) {
    return `${app.utils.getApiExampleURL()}/api/automation-webhooks/${automationId}`;
}

function isRecordAutomationTrigger(triggerType) {
    return triggerType === "record.create"
        || triggerType === "record.update"
        || triggerType === "record.delete";
}

function isI18nAutomationTrigger(triggerType) {
    return triggerType === "i18n.translation_missing"
        || triggerType === "i18n.locale_published"
        || triggerType === "i18n.translation_updated"
        || triggerType === "i18n.ai_translation_finished";
}

function fieldError(errorValue) {
    const message = extractErrorMessage(errorValue);
    if (!message) {
        return null;
    }

    return t.div({ className: "field-error txt-danger m-t-sm" }, message);
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

        try {
            return JSON.stringify(value);
        } catch (_) {
            return "";
        }
    }

    return String(value);
}
