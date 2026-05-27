const providerOptions = [
    { label: "OpenAI", value: "openai" },
    { label: "Gemini", value: "gemini" },
    { label: "Anthropic", value: "anthropic" },
    { label: "Custom", value: "custom" },
];

const providerBaseURLs = {
    openai: "https://api.openai.com/v1",
    gemini: "https://generativelanguage.googleapis.com/v1beta",
    anthropic: "https://api.anthropic.com/v1",
    custom: "",
};

export function defaultAIBaseURL(provider) {
    return providerBaseURLs[provider] || "";
}

function isProviderDefaultBaseURL(value) {
    const normalized = (value || "").trim();

    return Object.values(providerBaseURLs).filter(Boolean).includes(normalized);
}

export function aiAccordion(pageData) {
    const local = store({
        isFetchingModels: false,
        modelOptions: [],
    });

    function selectProvider(provider) {
        const ai = pageData.formSettings.ai;
        const previousProvider = ai.provider || providerOptions[0].value;
        const currentBaseURL = (ai.baseURL || "").trim();
        const previousDefaultBaseURL = defaultAIBaseURL(previousProvider);

        ai.provider = provider || providerOptions[0].value;
        local.modelOptions = [];

        if (!currentBaseURL || currentBaseURL == previousDefaultBaseURL || isProviderDefaultBaseURL(currentBaseURL)) {
            ai.baseURL = defaultAIBaseURL(ai.provider);
        }
    }

    function modelSelectOptions() {
        const useDisplayNameOnly = pageData.formSettings.ai.provider == "gemini";
        const options = local.modelOptions.map((model) => ({
            value: model.id,
            label: !model.label || model.label == model.id
                ? model.id
                : useDisplayNameOnly
                ? model.label
                : `${model.label} (${model.id})`,
        }));

        const current = pageData.formSettings.ai.model;
        if (current && !options.some((opt) => opt.value == current)) {
            options.unshift({ value: current, label: current });
        }

        return options;
    }

    async function refreshModels() {
        const ai = pageData.formSettings.ai;
        if (local.isFetchingModels || !ai.enabled) {
            return;
        }

        local.isFetchingModels = true;

        try {
            const result = await app.pb.send("/api/settings/ai/models", {
                method: "POST",
                body: app.utils.filterRedactedProps(ai),
            });

            local.modelOptions = result.models || [];
            if (!ai.model && local.modelOptions.length) {
                ai.model = local.modelOptions[0].id;
            }

            app.toasts.success("Models refreshed.");
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        local.isFetchingModels = false;
    }

    return t.details(
        {
            pbEvent: "aiSettingsAccordion",
            className: "accordion ai-settings-accordion",
            name: "settingsAccordion",
        },
        t.summary(
            null,
            t.i({ className: "ri-sparkling-line", ariaHidden: true }),
            t.span({ className: "txt" }, "AI provider"),
            t.div({ className: "flex-fill" }),
            () => {
                if (pageData.formSettings.ai.enabled) {
                    return t.span({ className: "label success" }, "Enabled");
                }
                return t.span({ className: "label" }, "Disabled");
            },
            () => {
                if (!app.utils.isEmpty(app.store.errors?.ai)) {
                    return t.i({
                        className: "ri-error-warning-fill txt-danger",
                        ariaDescription: app.attrs.tooltip("Has errors", "left"),
                    });
                }
            },
        ),
        t.div(
            { className: "grid sm" },
            t.div(
                { className: "col-lg-12" },
                t.div(
                    { className: "field" },
                    t.input({
                        id: "ai.enabled",
                        name: "ai.enabled",
                        type: "checkbox",
                        className: "switch",
                        checked: () => pageData.formSettings.ai.enabled || false,
                        onchange: (e) => (pageData.formSettings.ai.enabled = e.target.checked),
                    }),
                    t.label({ htmlFor: "ai.enabled" }, t.span({ className: "txt" }, "Enable")),
                ),
            ),
            t.div(
                { className: "col-lg-4" },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: "ai.provider" }, "Provider"),
                    app.components.select({
                        id: "ai.provider",
                        name: "ai.provider",
                        required: () => pageData.formSettings.ai.enabled,
                        disabled: () => !pageData.formSettings.ai.enabled,
                        options: providerOptions,
                        value: () => pageData.formSettings.ai.provider || providerOptions[0].value,
                        onchange: (selected) => {
                            selectProvider(selected?.[0]?.value);
                        },
                    }),
                ),
            ),
            t.div(
                { className: "col-lg-4" },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: "ai.apiKey" }, "API key"),
                    t.input({
                        id: "ai.apiKey",
                        name: "ai.apiKey",
                        type: "password",
                        autocomplete: "new-password",
                        required: () => pageData.formSettings.ai.enabled && !pageData.formSettings.ai.apiKey,
                        disabled: () => !pageData.formSettings.ai.enabled,
                        value: () => pageData.formSettings.ai.apiKey || "",
                        oninput: (e) => {
                            pageData.formSettings.ai.apiKey = e.target.value;
                            local.modelOptions = [];
                        },
                        onkeyup: (e) => {
                            if (e.key == "Backspace" && typeof pageData.formSettings.ai.apiKey === "undefined") {
                                pageData.formSettings.ai.apiKey = "";
                            }
                        },
                        placeholder: () => typeof pageData.formSettings.ai.apiKey !== "undefined" ? "" : "* * * * * *",
                    }),
                ),
            ),
            t.div(
                { className: "col-lg-4" },
                t.div(
                    { className: "field" },
                    t.div(
                        { className: "flex gap-xs m-b-xs" },
                        t.label({ htmlFor: "ai.model" }, "Default model"),
                        t.button(
                            {
                                type: "button",
                                className: () =>
                                    `btn btn-corner xs transparent secondary circle m-l-auto ${
                                        local.isFetchingModels ? "loading" : ""
                                    }`,
                                ariaLabel: app.attrs.tooltip("Refresh models"),
                                disabled: () => local.isFetchingModels || !pageData.formSettings.ai.enabled,
                                onclick: refreshModels,
                            },
                            t.i({ className: "ri-refresh-line", ariaHidden: true }),
                        ),
                    ),
                    () => {
                        if (local.modelOptions.length) {
                            return app.components.select({
                                id: "ai.model",
                                name: "ai.model",
                                disabled: () => !pageData.formSettings.ai.enabled,
                                options: () => modelSelectOptions(),
                                value: () => pageData.formSettings.ai.model || "",
                                onchange: (selected) => {
                                    pageData.formSettings.ai.model = selected?.[0]?.value || "";
                                },
                            });
                        }

                        return t.input({
                            id: "ai.model",
                            name: "ai.model",
                            type: "text",
                            maxlength: 255,
                            disabled: () => !pageData.formSettings.ai.enabled,
                            value: () => pageData.formSettings.ai.model || "",
                            oninput: (e) => (pageData.formSettings.ai.model = e.target.value),
                        });
                    },
                ),
            ),
            t.div(
                { className: "col-lg-12" },
                t.div(
                    { className: "field" },
                    t.label(
                        { htmlFor: "ai.baseURL" },
                        t.span({ className: "txt" }, "Base URL"),
                        t.i({
                            className: "ri-information-line link-faded",
                            ariaDescription: app.attrs.tooltip("Required for custom providers.", "right"),
                        }),
                    ),
                    t.input({
                        id: "ai.baseURL",
                        name: "ai.baseURL",
                        type: "url",
                        required: () =>
                            pageData.formSettings.ai.enabled && pageData.formSettings.ai.provider == "custom",
                        disabled: () => !pageData.formSettings.ai.enabled,
                        value: () => pageData.formSettings.ai.baseURL || "",
                        oninput: (e) => {
                            pageData.formSettings.ai.baseURL = e.target.value;
                            local.modelOptions = [];
                        },
                    }),
                ),
            ),
        ),
    );
}
