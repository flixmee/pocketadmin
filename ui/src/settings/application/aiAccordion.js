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

const providerEmbeddingModels = {
    openai: "text-embedding-3-small",
    gemini: "gemini-embedding-2",
    anthropic: "",
    custom: "",
};

export function defaultAIBaseURL(provider) {
    return providerBaseURLs[provider] || "";
}

export function defaultAIEmbeddingModel(provider) {
    return providerEmbeddingModels[provider] || "";
}

function embeddingModelPlaceholder(provider) {
    if (provider == "anthropic") {
        return "Not available";
    }
    if (provider == "custom") {
        return "Enter the provider embedding model";
    }

    return defaultAIEmbeddingModel(provider);
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
        const nextProvider = provider || providerOptions[0].value;
        const currentBaseURL = (ai.baseURL || "").trim();
        const previousDefaultBaseURL = defaultAIBaseURL(previousProvider);

        ai.provider = nextProvider;
        local.modelOptions = [];

        if (nextProvider != previousProvider) {
            ai.embeddingModel = defaultAIEmbeddingModel(nextProvider);
        }

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
                { className: "col-lg-4" },
                t.div(
                    { className: "field" },
                    t.label(
                        { htmlFor: "ai.embeddingModel" },
                        t.span({ className: "txt" }, "Embedding model"),
                        t.i({
                            className: "ri-information-line link-faded",
                            ariaDescription: app.attrs.tooltip(
                                "Used by Support search. Anthropic requires a custom knowledge provider for embeddings.",
                                "right",
                            ),
                        }),
                    ),
                    t.input({
                        id: "ai.embeddingModel",
                        name: "ai.embeddingModel",
                        type: "text",
                        maxlength: 255,
                        disabled: () =>
                            !pageData.formSettings.ai.enabled
                            || pageData.formSettings.ai.provider == "anthropic",
                        value: () => pageData.formSettings.ai.embeddingModel || "",
                        placeholder: () => embeddingModelPlaceholder(pageData.formSettings.ai.provider),
                        oninput: (e) => (pageData.formSettings.ai.embeddingModel = e.target.value),
                    }),
                    () => {
                        const provider = pageData.formSettings.ai.provider;
                        if (provider == "anthropic") {
                            return t.small(
                                { className: "txt-hint" },
                                "Anthropic doesn't provide an embedding API. Support search requires a custom knowledge provider.",
                            );
                        }
                        if (provider == "custom") {
                            return t.small(
                                { className: "txt-hint" },
                                "Enter the embedding model exposed by the custom OpenAI-compatible endpoint.",
                            );
                        }
                    },
                ),
            ),
            t.div(
                { className: "col-lg-8" },
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
            t.div(
                { className: "col-lg-12 m-t-sm" },
                t.h6(null, "Knowledge retrieval cache"),
                t.small(
                    { className: "txt-hint" },
                    "Caches recently retrieved Support chunks. Environment variables can override these values.",
                ),
            ),
            t.div(
                { className: "col-lg-4" },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: "ai.knowledgeCacheTTL" }, "Cache TTL (minutes)"),
                    t.input({
                        id: "ai.knowledgeCacheTTL",
                        name: "ai.knowledgeCacheTTL",
                        type: "number",
                        min: 1,
                        max: 1440,
                        step: 1,
                        disabled: () => !pageData.formSettings.ai.enabled,
                        value: () => pageData.formSettings.ai.knowledgeCacheTTL || 30,
                        oninput: (e) => pageData.formSettings.ai.knowledgeCacheTTL = parseInt(e.target.value, 10),
                    }),
                ),
            ),
            t.div(
                { className: "col-lg-4" },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: "ai.knowledgeCacheSimilarity" }, "Near-match threshold"),
                    t.input({
                        id: "ai.knowledgeCacheSimilarity",
                        name: "ai.knowledgeCacheSimilarity",
                        type: "number",
                        min: 0.5,
                        max: 1,
                        step: 0.01,
                        disabled: () => !pageData.formSettings.ai.enabled,
                        value: () => pageData.formSettings.ai.knowledgeCacheSimilarity || 0.92,
                        oninput: (e) =>
                            pageData.formSettings.ai.knowledgeCacheSimilarity = parseFloat(e.target.value),
                    }),
                ),
            ),
            t.div(
                { className: "col-lg-4" },
                t.div(
                    { className: "field" },
                    t.label({ htmlFor: "ai.knowledgeCacheMaxEntries" }, "Maximum entries"),
                    t.input({
                        id: "ai.knowledgeCacheMaxEntries",
                        name: "ai.knowledgeCacheMaxEntries",
                        type: "number",
                        min: 1,
                        max: 10000,
                        step: 1,
                        disabled: () => !pageData.formSettings.ai.enabled,
                        value: () => pageData.formSettings.ai.knowledgeCacheMaxEntries || 500,
                        oninput: (e) =>
                            pageData.formSettings.ai.knowledgeCacheMaxEntries = parseInt(e.target.value, 10),
                    }),
                ),
            ),
            t.div(
                { className: "col-lg-12" },
                t.div(
                    { className: "field" },
                    t.input({
                        id: "ai.knowledgeCacheAnswers",
                        name: "ai.knowledgeCacheAnswers",
                        type: "checkbox",
                        className: "switch",
                        disabled: () => !pageData.formSettings.ai.enabled,
                        checked: () => pageData.formSettings.ai.knowledgeCacheAnswers || false,
                        onchange: (e) => (pageData.formSettings.ai.knowledgeCacheAnswers = e.target.checked),
                    }),
                    t.label(
                        { htmlFor: "ai.knowledgeCacheAnswers" },
                        t.span({ className: "txt" }, "Reuse exact-match answers"),
                        t.small(
                            { className: "txt-hint" },
                            " Disabled by default; near matches always generate a fresh answer.",
                        ),
                    ),
                ),
            ),
        ),
    );
}
