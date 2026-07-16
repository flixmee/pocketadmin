export function collectionI18nOptionsTab(data) {
    const uniqueId = "collection_options_" + app.utils.randomString();

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
        ensureI18n().localizedFields = checked ? regularFields().map((field) => field.name) : [];
    }

    function tableFields() {
        return (data.collection.fields || []).filter((field) => {
            return field.id && !field.primaryKey && !!app.fieldTypes[field.type]?.view;
        });
    }

    function selectedTableFieldIds() {
        const fields = tableFields();
        const configured = data.collection.tableFields;
        const selected = new Set(
            Array.isArray(configured)
                ? configured
                : fields.filter((field) => !field.hidden).map((field) => field.id),
        );

        return fields.map((field) => field.id).filter((id) => selected.has(id));
    }

    function tableFieldCount() {
        return selectedTableFieldIds().length;
    }

    function toggleTableField(fieldId, checked) {
        const selected = selectedTableFieldIds();
        const index = selected.indexOf(fieldId);

        if (checked && index < 0) {
            selected.push(fieldId);
        } else if (!checked && index >= 0) {
            selected.splice(index, 1);
        }

        const selectedSet = new Set(selected);
        data.collection.tableFields = tableFields()
            .map((field) => field.id)
            .filter((id) => selectedSet.has(id));
    }

    function setAllTableFields(checked) {
        data.collection.tableFields = checked ? tableFields().map((field) => field.id) : [];
    }

    function localizationAccordion() {
        return t.details(
            {
                pbEvent: "collectionLocalizationAccordion",
                name: "collection-options",
                className: "accordion collection-options-accordion localization-options-accordion",
            },
            t.summary(
                {
                    onclick: (e) => {
                        if (!ensureI18n().enabled && !e.target.closest(".collection-option-summary-toggle")) {
                            e.preventDefault();
                        }
                    },
                },
                t.i({ className: "ri-translate-2", ariaHidden: true }),
                t.span({ className: "txt", textContent: "Localization" }),
                t.span({
                    className: () => `label m-l-auto ${ensureI18n().enabled ? "success" : ""}`,
                    textContent: () =>
                        ensureI18n().enabled
                            ? `${localizedFieldCount()}/${fieldCount()} fields`
                            : "Disabled",
                }),
                t.div(
                    {
                        className: "field collection-option-summary-toggle",
                        onclick: (e) => e.stopPropagation(),
                        onkeydown: (e) => e.stopPropagation(),
                    },
                    t.input({
                        id: uniqueId + "i18n_enabled",
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

                            e.target.closest("details").open = i18n.enabled;
                        },
                    }),
                    t.label({ htmlFor: uniqueId + "i18n_enabled" }, "Enable"),
                ),
            ),
            () => {
                if (!ensureI18n().enabled) {
                    return;
                }

                return t.div(
                    { className: "grid sm" },
                    t.div(
                        { className: "col-12" },
                        t.p(
                            { className: "txt-sm txt-hint" },
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
                                disabled: () => local.isLoadingLocales,
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
                                    disabled: () => !fieldCount() || localizedFieldCount() == fieldCount(),
                                    onclick: () => setAllLocalizedFields(true),
                                },
                                t.i({ className: "ri-checkbox-multiple-line", ariaHidden: true }),
                                t.span({ className: "txt" }, "Select all"),
                            ),
                            t.button(
                                {
                                    type: "button",
                                    className: "btn sm outline",
                                    disabled: () => !localizedFieldCount(),
                                    onclick: () => setAllLocalizedFields(false),
                                },
                                t.i({ className: "ri-close-line", ariaHidden: true }),
                                t.span({ className: "txt" }, "Clear"),
                            ),
                        ),
                        t.div(
                            { className: "field-list collection-option-field-list" },
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
                                        const inputId = `${uniqueId}localized_field_${field.name}`;

                                        return t.label(
                                            { className: "field-list-item checkbox" },
                                            t.input({
                                                id: inputId,
                                                type: "checkbox",
                                                checked: () =>
                                                    (ensureI18n().localizedFields || []).includes(field.name),
                                                onchange: (e) => toggleLocalizedField(field.name, e.target.checked),
                                            }),
                                            t.span(
                                                { className: "content" },
                                                t.span({ className: "txt txt-bold" }, field.name),
                                            ),
                                            t.span({ className: "label m-l-auto" }, field.type),
                                        );
                                    });
                                },
                            ),
                        ),
                    ),
                );
            },
        );
    }

    function tableFieldsAccordion() {
        return t.details(
            {
                pbEvent: "collectionTableFieldsAccordion",
                name: "collection-options",
                className: "accordion collection-options-accordion table-fields-options-accordion",
            },
            t.summary(
                null,
                t.i({ className: "ri-table-line", ariaHidden: true }),
                t.span({ className: "txt", textContent: "Table fields" }),
                t.span({
                    className: "label m-l-auto",
                    textContent: () => `${tableFieldCount()}/${tableFields().length} selected`,
                }),
            ),
            t.div(
                { className: "grid sm" },
                t.div(
                    { className: "col-12" },
                    t.p(
                        { className: "txt-sm txt-hint" },
                        "Choose the fields shown in the records table. The selection is saved with the collection in the database, and the primary key is always displayed.",
                    ),
                    t.div(
                        { className: "section-heading m-t-10" },
                        t.strong(null, "Displayed fields"),
                        t.div({ className: "flex-fill" }),
                        t.button(
                            {
                                type: "button",
                                className: "btn sm outline",
                                disabled: () => !tableFields().length || tableFieldCount() == tableFields().length,
                                onclick: () => setAllTableFields(true),
                            },
                            t.i({ className: "ri-checkbox-multiple-line", ariaHidden: true }),
                            t.span({ className: "txt" }, "Select all"),
                        ),
                        t.button(
                            {
                                type: "button",
                                className: "btn sm outline",
                                disabled: () => !tableFieldCount(),
                                onclick: () => setAllTableFields(false),
                            },
                            t.i({ className: "ri-close-line", ariaHidden: true }),
                            t.span({ className: "txt" }, "Clear"),
                        ),
                    ),
                    t.div(
                        { className: "field-list collection-option-field-list" },
                        t.div(
                            { className: "field-list-content" },
                            () => {
                                const fields = tableFields();
                                if (!fields.length) {
                                    return t.div(
                                        { className: "field-list-item txt-hint" },
                                        "No configurable table fields available.",
                                    );
                                }

                                return fields.map((field) => {
                                    const inputId = `${uniqueId}table_field_${field.id}`;

                                    return t.label(
                                        { className: "field-list-item checkbox" },
                                        t.input({
                                            id: inputId,
                                            type: "checkbox",
                                            checked: () => selectedTableFieldIds().includes(field.id),
                                            onchange: (e) => toggleTableField(field.id, e.target.checked),
                                        }),
                                        t.span(
                                            { className: "content" },
                                            t.span({ className: "txt txt-bold" }, field.name),
                                        ),
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

    return t.div(
        {
            pbEvent: "collectionI18nOptionsTab",
            className: "collection-tab-content collection-i18n-options-tab-content",
            onmount: () => {
                ensureI18n();
                loadLocales();
            },
        },
        localizationAccordion(),
        tableFieldsAccordion(),
    );
}
