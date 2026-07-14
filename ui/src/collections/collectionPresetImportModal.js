window.app = window.app || {};
window.app.modals = window.app.modals || {};

window.app.modals.openCollectionPresetImport = function(settings = {
    onsubmit: null,
}) {
    const modal = collectionPresetImportModal(settings || {});
    document.body.appendChild(modal);
    app.modals.open(modal);
};

function collectionPresetImportModal(settings) {
    const uniqueId = "collection_preset_" + app.utils.randomString();

    const data = store({
        presets: [],
        selectedPreset: "",
        prefix: "",
        preview: null,
        error: "",
        isLoading: false,
        isPreviewing: false,
        isImporting: false,
        previewRequest: 0,
        get isBusy() {
            return data.isLoading || data.isPreviewing || data.isImporting;
        },
        get selectedSummary() {
            return data.presets.find((preset) => preset.id == data.selectedPreset);
        },
        get canImport() {
            return !!data.preview?.canImport && !data.isBusy;
        },
    });

    let modal;

    function errorMessage(err, fallback) {
        return err?.response?.data?.preset?.message
            || err?.response?.data?.conflicts?.message
            || err?.response?.message
            || err?.message
            || fallback;
    }

    async function loadPresets() {
        data.isLoading = true;
        data.error = "";

        try {
            data.presets = await app.pb.send("/api/collection-presets", {
                method: "GET",
            });
            data.selectedPreset = data.presets[0]?.id || "";
            if (data.selectedPreset) {
                await loadPreview();
            }
        } catch (err) {
            data.error = errorMessage(err, "Failed to load collection presets.");
            app.checkApiError(err);
        }

        data.isLoading = false;
    }

    async function loadPreview() {
        if (!data.selectedPreset || data.isImporting) {
            return;
        }

        const request = ++data.previewRequest;
        data.isPreviewing = true;
        data.error = "";
        data.preview = null;

        try {
            const preview = await app.pb.send(
                `/api/collection-presets/${encodeURIComponent(data.selectedPreset)}/preview`,
                {
                    method: "POST",
                    body: { prefix: data.prefix },
                },
            );
            if (request == data.previewRequest) {
                data.preview = preview;
            }
        } catch (err) {
            if (request == data.previewRequest) {
                data.error = errorMessage(err, "Failed to preview the preset.");
                app.checkApiError(err);
            }
        }

        if (request == data.previewRequest) {
            data.isPreviewing = false;
        }
    }

    async function submit() {
        if (!data.canImport) {
            return;
        }

        data.isImporting = true;
        data.error = "";

        try {
            const importedNames = data.preview.collections.map((collection) => collection.name);
            await app.pb.send(
                `/api/collection-presets/${encodeURIComponent(data.selectedPreset)}/import`,
                {
                    method: "POST",
                    body: { prefix: data.prefix },
                },
            );
            await app.store.loadCollections();

            const importedCollections = app.store.collections.filter((collection) => {
                return importedNames.includes(collection.name);
            });
            settings.onsubmit?.(importedCollections);

            app.toasts.success(
                `Imported ${data.preview.preset.name} preset (${importedNames.length} collections).`,
            );
            data.isImporting = false;
            app.modals.close(modal);
            return;
        } catch (err) {
            data.error = errorMessage(err, "Failed to import the preset.");
            app.checkApiError(err);
        }

        data.isImporting = false;
    }

    function renderOptions() {
        return t.div(
            { className: "collection-preset-options" },
            t.div(
                { className: "field" },
                t.label(
                    { htmlFor: uniqueId + "_preset" },
                    t.span({ className: "txt" }, "Preset"),
                    t.i({
                        className: "ri-information-line link-hint",
                        tabIndex: 0,
                        ariaDescription: app.attrs.tooltip(
                            () => data.selectedSummary?.description || "Choose a built-in collection preset.",
                        ),
                    }),
                ),
                app.components.select({
                    id: uniqueId + "_preset",
                    required: true,
                    value: () => data.selectedPreset,
                    options: data.presets.map((preset) => ({
                        value: preset.id,
                        label: `${preset.name} (${preset.collectionCount})`,
                    })),
                    onchange: (options) => {
                        data.selectedPreset = options[0]?.value || "";
                        data.preview = null;
                        loadPreview();
                    },
                }),
            ),
            t.div(
                { className: "field" },
                t.label(
                    { htmlFor: uniqueId + "_prefix" },
                    t.span({ className: "txt" }, "Collection prefix (optional)"),
                    t.i({
                        className: "ri-information-line link-hint",
                        tabIndex: 0,
                        ariaDescription: app.attrs.tooltip(
                            "The prefix “blog” creates names such as blog_posts.",
                        ),
                    }),
                ),
                t.input({
                    id: uniqueId + "_prefix",
                    type: "text",
                    value: () => data.prefix,
                    placeholder: "e.g. blog",
                    disabled: () => data.isImporting,
                    oninput: (event) => {
                        data.prefix = event.target.value;
                        data.previewRequest++;
                        data.isPreviewing = false;
                        data.preview = null;
                        data.error = "";
                    },
                    onkeydown: (event) => {
                        if (event.key == "Enter") {
                            event.preventDefault();
                            loadPreview();
                        }
                    },
                }),
            ),
            t.button(
                {
                    type: "button",
                    className: () => `btn outline ${data.isPreviewing ? "loading" : ""}`,
                    disabled: () => !data.selectedPreset || data.isBusy,
                    onclick: () => loadPreview(),
                },
                t.i({ className: "ri-search-eye-line", ariaHidden: true }),
                t.span({ className: "txt" }, "Preview"),
            ),
        );
    }

    function renderPreview() {
        if (data.isPreviewing) {
            return t.div({ className: "collection-preset-loading" }, t.span({ className: "loader" }));
        }
        const preview = data.preview;
        if (!preview) {
            return t.div(
                { className: "alert info" },
                "Choose a preset and preview it before importing.",
            );
        }

        const collectionCards = preview.collections.map((collection) => {
            const fields = Array.isArray(collection.fields) ? collection.fields : [];
            const conflicts = preview.conflicts.filter((conflict) => conflict.collection == collection.name);
            const headerItems = [
                t.i({ className: "ri-folder-2-line", ariaHidden: true }),
                t.strong(null, collection.name),
            ];
            if (conflicts.length) {
                headerItems.push(
                    t.span(
                        {
                            className: "label warning collection-preset-conflict",
                            tabIndex: 0,
                            ariaDescription: app.attrs.tooltip(
                                conflicts.map((conflict) => conflict.message).join(" "),
                            ),
                        },
                        t.i({ className: "ri-error-warning-line", ariaHidden: true }),
                        "Conflict",
                    ),
                );
            }
            headerItems.push(
                t.span({ className: "label info m-l-auto" }, `${fields.length} fields`),
            );

            return t.section(
                {
                    className: `collection-preset-card ${conflicts.length ? "has-conflict" : ""}`,
                },
                t.div(
                    { className: "collection-preset-card-header" },
                    ...headerItems,
                ),
                t.div(
                    { className: "collection-preset-fields" },
                    ...fields.map((field) => {
                        return t.span(
                            { className: "collection-preset-field" },
                            t.span(null, field.name),
                            t.small({ className: "txt-hint" }, field.type),
                        );
                    }),
                ),
            );
        });

        const relationships = preview.relationships.map((relation) => {
            const items = [
                t.code(null, `${relation.collection}.${relation.field}`),
                t.i({ className: "ri-arrow-right-line txt-hint", ariaHidden: true }),
                t.code(null, relation.targetCollection),
            ];
            if (relation.external) {
                items.push(t.span({ className: "label warning" }, "existing"));
            }
            return t.div({ className: "list-item" }, ...items);
        });

        const content = [];
        if (!preview.conflicts.length) {
            content.push(
                t.div(
                    { className: "alert success" },
                    t.strong(null, "Ready to import. "),
                    `${preview.collections.length} collections will be created in one transaction.`,
                ),
            );
        }
        content.push(
            t.div({ className: "collection-preset-collections" }, ...collectionCards),
            t.section(
                { className: "collection-preset-relations" },
                t.h6(null, "Relationships"),
                t.div({ className: "list" }, ...relationships),
            ),
        );
        if (preview.warnings.length) {
            content.push(
                t.div(
                    { className: "alert warning" },
                    t.ul(null, ...preview.warnings.map((warning) => t.li(null, warning))),
                ),
            );
        }

        return t.div(
            { className: "collection-preset-preview" },
            ...content,
        );
    }

    modal = t.div(
        {
            pbEvent: "collectionPresetImportModal",
            className: "modal popup lg collection-preset-import-modal",
            onbeforeopen: () => {
                loadPresets();
            },
            onbeforeclose: () => !data.isImporting,
            onafterclose: (element) => element?.remove(),
        },
        t.header(
            { className: "modal-header" },
            t.div(
                null,
                t.h5({ className: "modal-title" }, "Import collection preset"),
                t.small({ className: "txt-hint" }, "Create a related collection set from a built-in template."),
            ),
            t.button(
                {
                    type: "button",
                    className: "btn sm secondary transparent circle modal-close-btn m-l-auto",
                    title: "Close",
                    disabled: () => data.isImporting,
                    onclick: () => app.modals.close(modal),
                },
                t.i({ className: "ri-close-line", ariaHidden: true }),
            ),
        ),
        t.div(
            { className: "modal-content" },
            () => {
                if (data.isLoading) {
                    return t.div({ className: "collection-preset-loading" }, t.span({ className: "loader lg" }));
                }
                if (!data.presets.length) {
                    return t.div({ className: "alert warning" }, data.error || "No collection presets are available.");
                }

                const content = [renderOptions()];
                if (data.error) {
                    content.push(t.div({ className: "alert danger" }, data.error));
                }
                content.push(renderPreview());
                return content;
            },
        ),
        t.footer(
            { className: "modal-footer" },
            t.button(
                {
                    type: "button",
                    className: "btn transparent m-r-auto",
                    disabled: () => data.isImporting,
                    onclick: () => app.modals.close(modal),
                },
                t.span({ className: "txt" }, "Cancel"),
            ),
            t.button(
                {
                    type: "button",
                    className: () => `btn ${data.isImporting ? "loading" : ""}`,
                    disabled: () => !data.canImport,
                    onclick: () => submit(),
                },
                t.i({ className: "ri-download-cloud-2-line", ariaHidden: true }),
                t.span({ className: "txt" }, "Import preset"),
            ),
        ),
    );

    return modal;
}
