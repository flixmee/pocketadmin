const TWEMOJI_BASE_URL = "https://cdn.jsdelivr.net/npm/twemoji@14.0.2/assets/svg/";

const LANGUAGE_OPTIONS = [
    { code: "en", name: "English", country: "us" },
    { code: "vi", name: "Tiếng Việt", country: "vn" },
    { code: "ko", name: "Korean", country: "kr" },
    { code: "zh", name: "Chinese", country: "cn" },
    { code: "ja", name: "Japanese", country: "jp" },
    { code: "fr", name: "French", country: "fr" },
    { code: "de", name: "German", country: "de" },
    { code: "es", name: "Spanish", country: "es" },
    { code: "pt", name: "Portuguese", country: "br" },
    { code: "it", name: "Italian", country: "it" },
    { code: "nl", name: "Dutch", country: "nl" },
    { code: "ru", name: "Russian", country: "ru" },
    { code: "ar", name: "Arabic", country: "sa" },
    { code: "hi", name: "Hindi", country: "in" },
    { code: "id", name: "Indonesian", country: "id" },
    { code: "th", name: "Thai", country: "th" },
    { code: "tr", name: "Turkish", country: "tr" },
    { code: "pl", name: "Polish", country: "pl" },
];

export function localesList(propsArg = {}) {
    const uniqueId = "locale_" + app.utils.randomString();

    const props = store({
        reset: null,
    });

    const watchers = app.utils.extendStore(props, propsArg);

    const data = store({
        isLoading: false,
        isSaving: false,
        isDeleting: {},
        showForm: false,
        locales: [],
        form: emptyForm(),
    });

    async function loadLocales() {
        data.isLoading = true;

        try {
            data.locales = await app.pb.send("/api/locales", {
                requestKey: "localesList.load",
            });
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        data.isLoading = false;
    }

    async function saveLocale(locale = null) {
        if (data.isSaving) {
            return;
        }

        data.isSaving = true;

        const payload = locale
            ? {
                code: locale.code,
                name: locale.label || locale.name,
                enabled: locale.enabled,
                is_default: locale.is_default,
            }
            : data.form;

        try {
            if (locale?.id) {
                await app.pb.send(`/api/locales/${locale.id}`, {
                    method: "PATCH",
                    body: payload,
                });
                app.toasts.success("Locale updated.");
            } else {
                await app.pb.send("/api/locales", {
                    method: "POST",
                    body: payload,
                });
                data.form = emptyForm();
                data.showForm = false;
                app.toasts.success("Locale created.");
            }

            app.store.errors = null;
            await loadLocales();
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err, false);
                app.toasts.error(err.message || "Failed to save locale.", { key: "localeSave" });
            }
        }

        data.isSaving = false;
    }

    async function deleteLocale(locale) {
        if (!locale?.id || data.isDeleting[locale.id]) {
            return;
        }

        data.isDeleting[locale.id] = true;

        try {
            await app.pb.send(`/api/locales/${locale.id}`, {
                method: "DELETE",
            });
            app.toasts.success("Locale deleted.");
            await loadLocales();
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        delete data.isDeleting[locale.id];
    }

    function confirmDelete(locale) {
        app.modals.confirm(
            `Do you really want to delete locale "${locale.code}"?`,
            () => deleteLocale(locale),
            null,
            { yesButton: "Delete", noButton: "Cancel" },
        );
    }

    function setDefault(locale) {
        locale.is_default = true;
        locale.enabled = true;
        saveLocale(locale);
    }

    function toggleEnabled(locale) {
        locale.enabled = !locale.enabled;
        saveLocale(locale);
    }

    function normalizeLocaleCode(code) {
        return String(code || "").trim().toLowerCase().replace("_", "-");
    }

    function selectedLanguage(code) {
        return LANGUAGE_OPTIONS.find((item) => item.code == normalizeLocaleCode(code));
    }

    function countryCodeForLocale(locale) {
        const code = normalizeLocaleCode(locale?.code || locale);
        const language = selectedLanguage(code);
        if (language) {
            return language.country;
        }

        const parts = code.split("-");
        return parts[1] || parts[0];
    }

    function twemojiFlagUrl(countryCode) {
        const code = String(countryCode || "").trim().toUpperCase();
        if (code.length != 2) {
            return "";
        }

        const codepoints = [...code].map((letter) => (letter.charCodeAt(0) + 127397).toString(16));
        return TWEMOJI_BASE_URL + codepoints.join("-") + ".svg";
    }

    function flag(localeOrCountry, label = "") {
        const countryCode = countryCodeForLocale(localeOrCountry);
        const src = twemojiFlagUrl(countryCode);

        if (!src) {
            return t.span({ className: "locale-flag locale-flag-empty", ariaHidden: true });
        }

        return t.img({
            className: "locale-flag",
            loading: "lazy",
            alt: label ? `${label} flag` : "",
            src,
        });
    }

    function languageLabel(locale) {
        const language = selectedLanguage(locale?.code);
        return locale?.label || locale?.name || language?.name || locale?.code || "";
    }

    function languageOptions() {
        const existingCodes = new Set((data.locales || []).map((locale) => normalizeLocaleCode(locale.code)));

        return LANGUAGE_OPTIONS.filter((language) => !existingCodes.has(language.code)).map((language) => ({
            value: language.code,
            label: () =>
                t.span(
                    { className: "locale-option" },
                    flag(language.country, language.name),
                    t.span({ className: "txt" }, language.name),
                    t.span({ className: "txt-hint txt-code m-l-auto" }, language.code),
                ),
            selected: () =>
                t.span(
                    { className: "locale-option selected" },
                    flag(language.country, language.name),
                    t.span({ className: "txt" }, language.name),
                ),
        }));
    }

    function selectLanguage(code) {
        const language = selectedLanguage(code);

        data.form.code = language?.code || "";
        data.form.name = language?.name || "";
    }

    function addButton() {
        return t.button(
            {
                type: "button",
                className: "btn secondary locales-add-toggle",
                onclick: () => data.showForm = !data.showForm,
            },
            t.span({ className: "txt" }, "Add new language"),
            t.i({
                className: () => data.showForm ? "ri-arrow-up-s-line" : "ri-arrow-down-s-line",
                ariaHidden: true,
            }),
        );
    }

    return t.div(
        {
            pbEvent: "localesList",
            className: "locales-list",
            onmount: () => {
                loadLocales();
                watchers.push(watch(() => props.reset, () => loadLocales()));
            },
            onunmount: () => watchers.forEach((w) => w?.unwatch()),
        },
        t.div(
            { className: "locales-table" },
            t.div(
                { className: "locales-table-row locales-table-head" },
                t.div(null, "Language"),
                t.div(null, "Code"),
                t.div(null, "Default"),
                t.div({ className: "txt-right" }, "Actions"),
            ),
            t.div(
                {
                    hidden: () => !data.isLoading || data.locales.length,
                    className: "locales-table-row",
                },
                t.div({ className: "skeleton-loader" }),
                t.div({ className: "skeleton-loader" }),
                t.div({ className: "skeleton-loader" }),
                t.div({ className: "skeleton-loader" }),
            ),
            () =>
                data.locales.map((locale) => {
                    return t.div(
                        { className: () => `locales-table-row ${data.isLoading ? "faded" : ""}` },
                        t.div(
                            { className: "locale-language-cell" },
                            flag(locale, languageLabel(locale)),
                            t.div(
                                { className: "min-width-0" },
                                t.div({ className: "txt-bold txt-ellipsis" }, () => languageLabel(locale)),
                                t.div(
                                    { className: "txt-sm txt-hint" },
                                    () => locale.enabled ? "Enabled" : "Disabled",
                                ),
                            ),
                        ),
                        t.div({ className: "txt-code" }, () => locale.code),
                        t.div(
                            null,
                            t.button(
                                {
                                    type: "button",
                                    ariaLabel: app.attrs.tooltip(
                                        locale.is_default ? "Default locale" : "Make default",
                                    ),
                                    className: () =>
                                        `btn sm locale-default-btn ${locale.is_default ? "success" : "secondary"}`,
                                    disabled: () => data.isSaving || locale.is_default || !locale.enabled,
                                    onclick: () => setDefault(locale),
                                },
                                t.i({ className: "ri-check-line", ariaHidden: true }),
                            ),
                        ),
                        t.nav(
                            { className: "locale-row-actions" },
                            t.button(
                                {
                                    type: "button",
                                    ariaLabel: app.attrs.tooltip(locale.enabled ? "Disable" : "Enable"),
                                    className: "btn sm circle secondary transparent",
                                    disabled: () => data.isSaving || locale.is_default,
                                    onclick: () => toggleEnabled(locale),
                                },
                                t.i({
                                    className: () => locale.enabled ? "ri-pause-line" : "ri-play-line",
                                    ariaHidden: true,
                                }),
                            ),
                            t.button(
                                {
                                    type: "button",
                                    ariaLabel: app.attrs.tooltip("Delete"),
                                    className: () =>
                                        `btn sm circle danger locale-delete-btn ${
                                            data.isDeleting[locale.id] ? "loading" : ""
                                        }`,
                                    disabled: () => data.isSaving || locale.is_default,
                                    onclick: () => confirmDelete(locale),
                                },
                                t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
                            ),
                        ),
                    );
                }),
        ),
        t.div({ className: "m-t-base" }, addButton()),
        t.div(
            { hidden: () => !data.showForm, className: "locales-add-panel" },
            t.div(
                { className: "grid sm" },
                t.div(
                    { className: "col-md-6" },
                    t.div(
                        { className: "field" },
                        t.label({ textContent: "Language" }),
                        app.components.select({
                            placeholder: "Select a language",
                            searchThreshold: 1,
                            options: () => languageOptions(),
                            value: () => data.form.code,
                            onchange: (options) => selectLanguage(options?.[0]?.value),
                        }),
                    ),
                ),
                t.div(
                    { className: "col-md-6" },
                    t.div(
                        { className: "field" },
                        t.label({ textContent: "Display name" }),
                        t.input({
                            type: "text",
                            placeholder: "Display name",
                            value: () => data.form.name,
                            oninput: (e) => data.form.name = e.target.value,
                        }),
                    ),
                ),
                t.div(
                    { className: "col-md-3" },
                    t.div(
                        { className: "field" },
                        t.label({ textContent: "Code" }),
                        t.input({
                            type: "text",
                            placeholder: "Code",
                            value: () => data.form.code,
                            oninput: (e) => data.form.code = normalizeLocaleCode(e.target.value),
                        }),
                    ),
                ),
                t.div(
                    { className: "col-md-3" },
                    t.div(
                        { className: "field" },
                        t.input({
                            id: uniqueId + "enabled",
                            type: "checkbox",
                            className: "switch sm",
                            checked: () => data.form.enabled,
                            onchange: (e) => data.form.enabled = e.target.checked,
                        }),
                        t.label({ htmlFor: uniqueId + "enabled" }, "Enabled"),
                    ),
                ),
                t.div(
                    { className: "col-md-3" },
                    t.div(
                        { className: "field" },
                        t.input({
                            id: uniqueId + "default",
                            type: "checkbox",
                            className: "switch sm",
                            checked: () => data.form.is_default,
                            onchange: (e) => {
                                data.form.is_default = e.target.checked;
                                if (data.form.is_default) {
                                    data.form.enabled = true;
                                }
                            },
                        }),
                        t.label({ htmlFor: uniqueId + "default" }, "Default"),
                    ),
                ),
                t.div(
                    { className: "col-md-3 flex align-end" },
                    t.button(
                        {
                            type: "button",
                            className: () => `btn block ${data.isSaving ? "loading" : ""}`,
                            disabled: () => data.isSaving || !data.form.code || !data.form.name,
                            onclick: () => saveLocale(),
                        },
                        t.i({ className: "ri-add-line", ariaHidden: true }),
                        t.span({ className: "txt" }, "Add language"),
                    ),
                ),
            ),
        ),
    );
}

function emptyForm() {
    return {
        code: "",
        name: "",
        enabled: true,
        is_default: false,
    };
}
