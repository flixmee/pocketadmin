import { openAutomationDryRunModal } from "./automationDryRunModal";
import { openAutomationRunsModal } from "./automationRunsList";
import { buildAutomationStepsPayload, normalizeAutomationEditorSteps, stepEditor } from "./stepEditor";

const automationTriggerOptions = [
    { value: "manual", label: "Manual" },
    { value: "webhook", label: "Webhook" },
    { value: "schedule.cron", label: "Scheduled cron" },
    { value: "record.beforeCreate", label: "Before record create" },
    { value: "record.beforeUpdate", label: "Before record update" },
    { value: "record.create", label: "Record create" },
    { value: "record.update", label: "Record update" },
    { value: "record.delete", label: "Record delete" },
    { value: "i18n.translation_missing", label: "Translation missing" },
    { value: "i18n.locale_published", label: "Locale published" },
    { value: "i18n.translation_updated", label: "Translation updated" },
    { value: "i18n.ai_translation_finished", label: "AI translation finished" },
];

const customTagOptionValue = "__pocketadmin_custom_tag__";

export function pageAutomationUpsert(route) {
    const automationId = route.params?.id || "";
    const isNew = !automationId;
    const formId = "automation_page_upsert_" + app.utils.randomString();
    app.store.title = isNew ? "Create automation" : "Edit automation";

    const initialForm = normalizeAutomationForm(null);
    const data = store({
        isLoading: !isNew,
        isLoadingTags: false,
        isSaving: false,
        automation: null,
        automationTags: [],
        form: initialForm,
        initialSerialized: JSON.stringify(initialForm),
        get title() {
            return isNew ? "Create automation" : data.form.name || "Edit automation";
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

    function hydrateForm(automation) {
        const nextForm = normalizeAutomationForm(automation);
        data.automation = automation;
        data.form = cloneAutomationForm(nextForm);
        data.initialSerialized = JSON.stringify(nextForm);
        app.store.title = isNew ? "Create automation" : data.form.name || "Edit automation";
    }

    async function loadPageData() {
        await Promise.all([
            loadAutomation(),
            loadAutomationTags(),
        ]);
    }

    async function loadAutomation() {
        if (isNew) {
            return;
        }

        data.isLoading = true;
        try {
            hydrateForm(
                await app.pb.send(`/api/automations/${automationId}`, {
                    requestKey: "automationUpsert.load",
                }),
            );
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }
        data.isLoading = false;
    }

    async function loadAutomationTags() {
        data.isLoadingTags = true;
        try {
            const automations = await app.pb.send("/api/automations", {
                requestKey: "automationUpsert.loadTags",
            });
            data.automationTags = uniqueAutomationTags(automations);
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }
        data.isLoadingTags = false;
    }

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

    function tagOptions(selectedValue = "") {
        const tags = new Set(data.automationTags);
        const selectedTag = normalizeTag(selectedValue);
        if (selectedTag && selectedTag !== customTagOptionValue) {
            tags.add(selectedTag);
        }

        const options = [...tags]
            .sort((a, b) => a.localeCompare(b))
            .map((tag) => ({
                value: tag,
                label: tag,
            }));

        options.push({
            value: customTagOptionValue,
            label: "Other...",
        });

        return options;
    }

    function openCustomTagDialog() {
        const dialogFormId = "automation_custom_tag_" + app.utils.randomString();
        const customTagData = store({
            value: normalizeTag(data.form.tag),
        });

        let modal;

        function submit() {
            const tag = normalizeTag(customTagData.value);
            if (!tag) {
                return;
            }

            data.form.tag = tag;
            app.modals.close(modal, true);
        }

        modal = t.div(
            {
                className: "modal popup sm automation-tag-modal",
                onafterclose: (el) => el?.remove(),
            },
            t.header(
                { className: "modal-header" },
                t.h5({ className: "m-auto" }, "Custom tag"),
            ),
            t.form(
                {
                    id: dialogFormId,
                    className: "modal-content",
                    onsubmit: (e) => {
                        e.preventDefault();
                        submit();
                    },
                },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: dialogFormId + "_tag" }, "Tag"),
                    t.input({
                        id: dialogFormId + "_tag",
                        type: "text",
                        maxlength: 255,
                        required: true,
                        autofocus: true,
                        value: () => customTagData.value,
                        oninput: (e) => (customTagData.value = e.target.value),
                    }),
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
                    t.span({ className: "txt" }, "Cancel"),
                ),
                t.button(
                    {
                        type: "submit",
                        "html-form": dialogFormId,
                        className: "btn",
                    },
                    t.span({ className: "txt" }, "Apply"),
                ),
            ),
        );

        document.body.appendChild(modal);
        app.modals.open(modal);
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
            const saved = isNew
                ? await app.pb.send("/api/automations", {
                    method: "POST",
                    body: payload,
                })
                : await app.pb.send(`/api/automations/${automationId}`, {
                    method: "PATCH",
                    body: payload,
                });

            hydrateForm(saved);
            app.toasts.success(isNew ? "Automation created." : "Automation updated.");
            if (isNew) {
                window.location.hash = `#/automations/${saved.id}`;
            }
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        data.isSaving = false;
    }

    function openRunsModal() {
        if (!data.automation?.id || data.isSaving) {
            return;
        }

        openAutomationRunsModal(data.automation);
    }

    function openDryRunModal() {
        if (!data.automation?.id || data.isSaving) {
            return;
        }

        openAutomationDryRunModal(data.automation);
    }

    async function exportTemplate() {
        if (!data.automation?.id || data.isSaving) {
            return;
        }

        data.isSaving = true;
        try {
            await app.pb.send(`/api/automations/${data.automation.id}/export-template`, {
                method: "POST",
                body: {
                    name: data.form.name,
                    description: data.form.notes,
                },
            });
            app.toasts.success("Workflow template exported.");
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }
        data.isSaving = false;
    }

    return t.div(
        {
            pbEvent: "pageAutomationUpsert",
            className: "page page-automation-upsert",
            onmount: loadPageData,
            onunmount: () => {
                if (app.store.errors) {
                    app.store.errors = null;
                }
            },
        },
        t.div(
            { className: "page-content full-height" },
            t.header(
                { className: "page-header" },
                t.nav(
                    { className: "breadcrumbs" },
                    t.a({ href: "#/automations", className: "breadcrumb-item" }, "Automations"),
                    t.div({ className: "breadcrumb-item" }, () => data.title),
                ),
                t.div({ className: "flex-fill" }),
                t.a(
                    { href: "#/automations", className: "btn secondary transparent" },
                    t.i({ className: "ri-arrow-left-line", ariaHidden: true }),
                    t.span({ className: "txt" }, "Back"),
                ),
            ),
            t.div(
                { className: "m-b-base" },
                t.div(
                    {
                        hidden: () => !data.isLoading,
                        className: "skeleton-loader",
                    },
                ),
                t.div(
                    { hidden: () => data.isLoading },
                    ...renderAutomationForm(),
                ),
            ),
            t.footer({ className: "page-footer" }, app.components.credits()),
        ),
    );

    function renderAutomationForm() {
        return [
            t.div(
                { className: "flex gap-10 flex-wrap m-b-base align-items-center" },
                t.div(
                    { className: "content block" },
                    t.h3({ className: "m-b-xs automation-page-title" }, () => data.title),
                    t.div(
                        { className: "txt-sm txt-hint automation-page-desc" },
                        "Build, validate, preview, and publish automation workflows from one page.",
                    ),
                ),
                t.div({ className: "m-l-auto" }),
                t.div(
                    { className: "flex gap-5 flex-wrap" },
                    () => {
                        if (isNew) {
                            return null;
                        }

                        return t.button(
                            {
                                type: "button",
                                className: "btn secondary transparent pill",
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
                                className: "btn secondary transparent pill",
                                disabled: () => data.isSaving,
                                onclick: exportTemplate,
                            },
                            t.i({ className: "ri-file-upload-line", ariaHidden: true }),
                            t.span({ className: "txt" }, "Export"),
                        );
                    },
                    () => {
                        if (isNew) {
                            return null;
                        }

                        return t.button(
                            {
                                type: "button",
                                className: "btn secondary transparent pill",
                                disabled: () => data.isSaving,
                                onclick: openRunsModal,
                            },
                            t.i({ className: "ri-history-line", ariaHidden: true }),
                            t.span({ className: "txt" }, "Recent runs"),
                        );
                    },
                    t.button(
                        {
                            type: "submit",
                            "html-form": formId,
                            className: () => `btn pill ${data.isSaving ? "loading" : ""}`,
                            disabled: () => !data.canSave,
                        },
                        t.span({ className: "txt" }, () => data.submitLabel),
                    ),
                ),
            ),
            t.form(
                {
                    id: formId,
                    className: "automation-upsert-page-form",
                    inert: () => data.isSaving,
                    onsubmit: (e) => {
                        e.preventDefault();
                        save();
                    },
                },
                t.div(
                    { className: "automation-builder-card automation-config-card m-b-base" },
                    t.div(
                        { className: "grid" },
                        t.div(
                            { className: "col-md-6" },
                            t.div(
                                { className: "field" },
                                t.label({ htmlFor: formId + "_name", className: "automation-field-label" }, "Name"),
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
                            { className: "col-md-3" },
                            t.div(
                                { className: "field" },
                                t.label({ htmlFor: formId + "_tag", className: "automation-field-label" }, "Tag"),
                                app.components.select({
                                    id: formId + "_tag",
                                    name: "tag",
                                    value: () => data.form.tag,
                                    options: () => tagOptions(data.form.tag),
                                    placeholder: () => data.isLoadingTags ? "Loading tags..." : "Select existing tag",
                                    searchThreshold: 8,
                                    disabled: () => data.isLoadingTags || tagOptions(data.form.tag).length === 0,
                                    onchange: (selected) => {
                                        const selectedValue = selected?.[0]?.value || "";
                                        if (selectedValue === customTagOptionValue) {
                                            openCustomTagDialog();
                                            return;
                                        }

                                        data.form.tag = selectedValue;
                                    },
                                }),
                            ),
                            () => fieldError(app.store.errors?.tag),
                        ),
                        t.div(
                            { className: "col-md-3" },
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
                                t.label(
                                    { htmlFor: formId + "_active", className: "automation-field-label" },
                                    t.span({ className: "txt" }, "Active"),
                                ),
                            ),
                        ),
                        t.div(
                            { className: "col-md-3" },
                            t.div(
                                { className: "field m-t-lg" },
                                t.input({
                                    id: formId + "_notifyOnCompletion",
                                    name: "notifyOnCompletion",
                                    type: "checkbox",
                                    className: "switch",
                                    checked: () => data.form.notifyOnCompletion,
                                    onchange: (e) => (data.form.notifyOnCompletion = e.target.checked),
                                }),
                                t.label(
                                    {
                                        htmlFor: formId + "_notifyOnCompletion",
                                        className: "automation-field-label",
                                    },
                                    t.span({ className: "txt" }, "Notify admins"),
                                ),
                            ),
                            t.div(
                                { className: "field-help automation-field-desc" },
                                "Create admin notifications when this automation succeeds or fails.",
                            ),
                        ),
                        t.div(
                            { className: "col-md-6" },
                            t.div(
                                { className: "field" },
                                t.label(
                                    { htmlFor: formId + "_trigger", className: "automation-field-label" },
                                    "Trigger",
                                ),
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
                                t.label(
                                    { htmlFor: formId + "_collectionRef", className: "automation-field-label" },
                                    "Target collection",
                                ),
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
                                t.label(
                                    { htmlFor: formId + "_cronExpr", className: "automation-field-label" },
                                    "Cron expression",
                                ),
                                t.input({
                                    id: formId + "_cronExpr",
                                    name: "cronExpr",
                                    type: "text",
                                    placeholder: "0 * * * *",
                                    value: () => data.form.cronExpr,
                                    oninput: (e) => (data.form.cronExpr = e.target.value),
                                }),
                            ),
                            t.div(
                                { className: "field-help automation-field-desc" },
                                "Use standard cron syntax for scheduled automations.",
                            ),
                            () => fieldError(app.store.errors?.cronExpr),
                        ),
                        t.div(
                            {
                                className: "col-lg-12",
                                hidden: () => !data.isWebhookTrigger,
                            },
                            t.div(
                                { className: "field" },
                                t.label({ className: "automation-field-label" }, "Webhook endpoint"),
                                () => {
                                    if (!data.automation?.id) {
                                        return t.div(
                                            { className: "txt-sm txt-hint automation-field-desc p-10" },
                                            "Save the automation first to generate its stable webhook endpoint.",
                                        );
                                    }

                                    return t.div(
                                        { className: "flex gap-10 flex-wrap p-10" },
                                        t.code(null, webhookURL(data.automation.id)),
                                        app.components.copyButton(() => webhookURL(data.automation.id)),
                                    );
                                },
                            ),
                            t.div(
                                { className: "field-help automation-field-desc" },
                                "Send a POST request to this endpoint. Templates can read incoming request values.",
                            ),
                        ),
                        t.div(
                            { className: "col-lg-12" },
                            t.div(
                                { className: "field" },
                                t.label({ htmlFor: formId + "_notes", className: "automation-field-label" }, "Notes"),
                                t.textarea({
                                    id: formId + "_notes",
                                    name: "notes",
                                    rows: 2,
                                    placeholder: "Optional internal notes for operators.",
                                    value: () => data.form.notes,
                                    oninput: (e) => (data.form.notes = e.target.value),
                                }),
                            ),
                            () => fieldError(app.store.errors?.notes),
                        ),
                    ),
                ),
                t.div(
                    { className: "automation-builder-workspace" },
                    stepEditor({
                        steps: () => data.form.steps,
                        errors: () => app.store.errors?.steps,
                        triggerType: () => data.form.triggerType,
                        triggerCollectionRef: () => data.form.collectionRef,
                        onchange: (steps) => {
                            data.form.steps = steps;
                        },
                    }),
                ),
            ),
        ];
    }
}

function normalizeAutomationForm(automation = null) {
    return {
        name: automation?.name || "",
        tag: automation?.tag || "",
        active: !!automation?.active,
        notifyOnCompletion: !!automation?.notifyOnCompletion,
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

function normalizeTag(value) {
    return String(value || "").trim();
}

function uniqueAutomationTags(automations) {
    return [...new Set((automations || []).map((automation) => normalizeTag(automation.tag)).filter(Boolean))];
}

function buildAutomationPayload(form) {
    return {
        name: form.name.trim(),
        tag: form.tag.trim(),
        active: !!form.active,
        notifyOnCompletion: !!form.notifyOnCompletion,
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
    return triggerType === "record.beforeCreate"
        || triggerType === "record.beforeUpdate"
        || triggerType === "record.create"
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
