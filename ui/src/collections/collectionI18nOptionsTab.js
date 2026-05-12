export function collectionI18nOptionsTab(data) {
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
                t.label(
                    { className: "checkbox" },
                    t.input({
                        type: "checkbox",
                        checked: () => !!ensureI18n().enabled,
                        onchange: (e) => {
                            const i18n = ensureI18n();
                            i18n.enabled = e.target.checked;
                            if (i18n.enabled && !i18n.defaultLocale) {
                                i18n.defaultLocale = local.locales.find((locale) => locale.is_default)?.code || "en";
                            }
                        },
                    }),
                    t.span({ className: "txt" }, "Enable localization"),
                ),
            ),
            t.div(
                { className: "col-12" },
                t.div(
                    { className: "field" },
                    t.label({ textContent: "Default locale" }),
                    app.components.select({
                        placeholder: "- Select locale -",
                        options: () => {
                            return (local.locales || []).map((locale) => ({
                                value: locale.code,
                                label: `${locale.code} - ${locale.label || locale.name}`,
                            }));
                        },
                        value: () => ensureI18n().defaultLocale || "",
                        disabled: () => !ensureI18n().enabled || local.isLoadingLocales,
                        onchange: (options) => {
                            ensureI18n().defaultLocale = options?.[0]?.value || "";
                        },
                    }),
                ),
            ),
            t.div(
                { className: "col-12" },
                t.div(
                    { className: "field" },
                    t.label({ textContent: "Localized fields" }),
                    t.div(
                        { className: "list compact" },
                        () => {
                            const fields = regularFields();
                            if (!fields.length) {
                                return t.div({ className: "list-item txt-hint" }, "No editable fields available.");
                            }

                            return fields.map((field) => {
                                return t.label(
                                    { className: "list-item checkbox" },
                                    t.input({
                                        type: "checkbox",
                                        disabled: () => !ensureI18n().enabled,
                                        checked: () => (ensureI18n().localizedFields || []).includes(field.name),
                                        onchange: (e) => toggleLocalizedField(field.name, e.target.checked),
                                    }),
                                    t.span({ className: "txt" }, field.name),
                                    t.small({ className: "txt-hint m-l-5" }, field.type),
                                );
                            });
                        },
                    ),
                ),
            ),
        ),
    );
}
