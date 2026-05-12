export function collectionI18nOptionsTab(data) {
    const uniqueId = "i18n_" + app.utils.randomString();

    const local = store({
        isLoadingLocales: false,
        locales: [],
    });

    async function loadLocales() {
        local.isLoadingLocales = true;

        try {
            local.locales = await app.pb.send("/api/locales", {
                requestKey: "collectionI18nOptionsTab.locales",
            });
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        }

        local.isLoadingLocales = false;
    }

    function ensureI18n() {
        data.collection.i18n = data.collection.i18n || {
            enabled: false,
            defaultLocale: "",
            localizedFields: [],
        };
        data.collection.i18n.localizedFields = data.collection.i18n.localizedFields || [];
        return data.collection.i18n;
    }

    function regularFields() {
        return (data.collection.fields || []).filter((field) => {
            return !field.system && field.name && field.type != "password";
        });
    }

    function localeOptions() {
        const i18n = ensureI18n();
        const locales = (local.locales || []).filter((locale) => {
            return locale.enabled || locale.code == i18n.defaultLocale;
        });

        return locales.map((locale) => ({
            value: locale.code,
            label: () =>
                t.div(
                    { className: "flex gap-5" },
                    t.span({ className: "txt-code" }, locale.code),
                    t.span({ className: "txt-ellipsis" }, locale.label || locale.name),
                    t.span(
                        {
                            hidden: () => !locale.is_default,
                            className: "label primary m-l-auto",
                        },
                        "Default",
                    ),
                    t.span(
                        {
                            hidden: () => locale.enabled,
                            className: "label warning m-l-auto",
                        },
                        "Disabled",
                    ),
                ),
            selected: () =>
                t.span(null, locale.code, locale.label || locale.name ? ` - ${locale.label || locale.name}` : ""),
        }));
    }

    function fieldCount() {
        return regularFields().length;
    }

    function localizedFieldCount() {
        const fieldNames = new Set(regularFields().map((field) => field.name));
        return (ensureI18n().localizedFields || []).filter((name) => fieldNames.has(name)).length;
    }

    function toggleLocalizedField(fieldName, checked) {
        const i18n = ensureI18n();
        const fields = i18n.localizedFields || [];

        if (checked && !fields.includes(fieldName)) {
            fields.push(fieldName);
        } else if (!checked) {
            const index = fields.indexOf(fieldName);
            if (index >= 0) {
                fields.splice(index, 1);
            }
        }
    }

    function setAllLocalizedFields(checked) {
        const i18n = ensureI18n();
        i18n.localizedFields = checked ? regularFields().map((field) => field.name) : [];
    }

    return t.div(
        {
            pbEvent: "collectionI18nOptionsTab",
            className: "collection-tab-content collection-i18n-options-tab-content",
            onmount: () => {
                ensureI18n();
                loadLocales();
            },
        },
        t.div(
            { className: "grid" },
            t.div(
                { className: "col-12" },
                t.div(
                    { className: "section-heading" },
                    t.strong(null, "Localization"),
                    t.div({ className: "flex-fill" }),
                    t.div(
                        { className: "field" },
                        t.input({
                            id: uniqueId + "enabled",
                            name: "i18n.enabled",
                            type: "checkbox",
                            className: "switch sm",
                            checked: () => !!ensureI18n().enabled,
                            onchange: (e) => {
                                const i18n = ensureI18n();
                                i18n.enabled = e.target.checked;
                                if (i18n.enabled && !i18n.defaultLocale) {
                                    i18n.defaultLocale = local.locales.find((locale) => locale.is_default)?.code
                                        || local.locales.find((locale) => locale.enabled)?.code
                                        || "en";
                                }
                            },
                        }),
                        t.label({ htmlFor: uniqueId + "enabled" }, "Enable"),
                    ),
                ),
                t.p(
                    { className: "txt-sm txt-hint m-b-sm" },
                    "Create per-locale records for this collection and choose which regular fields editors should translate.",
                ),
            ),
            t.div(
                { className: "col-12 col-sm-6" },
                t.div(
                    { className: "field" },
                    t.label({ textContent: "Default locale" }),
                    app.components.select({
                        placeholder: "- Select locale -",
                        options: () => localeOptions(),
                        value: () => ensureI18n().defaultLocale || "",
                        disabled: () => !ensureI18n().enabled || local.isLoadingLocales,
                        onchange: (options) => {
                            ensureI18n().defaultLocale = options?.[0]?.value || "";
                        },
                    }),
                    t.div(
                        {
                            hidden: () => local.isLoadingLocales || local.locales.length,
                            className: "field-help txt-warning",
                        },
                        "No locales configured yet.",
                    ),
                ),
            ),
            t.div(
                { className: "col-12" },
                t.div(
                    { className: "section-heading m-t-10" },
                    t.strong(null, "Localized fields"),
                    t.span({ className: "label" }, () => `${localizedFieldCount()}/${fieldCount()} selected`),
                    t.div({ className: "flex-fill" }),
                    t.button(
                        {
                            type: "button",
                            className: "btn sm outline",
                            disabled: () =>
                                !ensureI18n().enabled || !fieldCount() || localizedFieldCount() == fieldCount(),
                            onclick: () => setAllLocalizedFields(true),
                        },
                        t.i({ className: "ri-checkbox-multiple-line", ariaHidden: true }),
                        t.span({ className: "txt" }, "Select all"),
                    ),
                    t.button(
                        {
                            type: "button",
                            className: "btn sm outline",
                            disabled: () => !ensureI18n().enabled || !localizedFieldCount(),
                            onclick: () => setAllLocalizedFields(false),
                        },
                        t.i({ className: "ri-close-line", ariaHidden: true }),
                        t.span({ className: "txt" }, "Clear"),
                    ),
                ),
                t.div(
                    { className: "field-list" },
                    t.div(
                        { className: "field-list-content" },
                        () => {
                            const fields = regularFields();
                            if (!fields.length) {
                                return t.div(
                                    { className: "field-list-item txt-hint" },
                                    "No editable fields available.",
                                );
                            }

                            return fields.map((field) => {
                                const inputId = `${uniqueId}field_${field.name}`;

                                return t.label(
                                    { className: "field-list-item checkbox" },
                                    t.input({
                                        id: inputId,
                                        type: "checkbox",
                                        disabled: () => !ensureI18n().enabled,
                                        checked: () => (ensureI18n().localizedFields || []).includes(field.name),
                                        onchange: (e) => toggleLocalizedField(field.name, e.target.checked),
                                    }),
                                    t.span({ className: "content" }, t.span({ className: "txt txt-bold" }, field.name)),
                                    t.span({ className: "label m-l-auto" }, field.type),
                                );
                            });
                        },
                    ),
                ),
            ),
        ),
    );
}
