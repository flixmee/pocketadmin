import { getMediaFileURL, getMediaSelectionLabel, normalizeMediaFileURL } from "./utils";

window.app = window.app || {};
window.app.modals = window.app.modals || {};

const mediasCollectionName = "_medias";
const listRequestKey = "mediaPicker.list";
const viewModeStorageKey = "pbMediaPickerViewMode";

const defaultSettings = {
    selectedPaths: [],
    maxSelect: 1,
    allowFolders: false,
    mimeTypes: [],
    btnText: "Set selection",
    onselect: function(values) {},
};

window.app.modals.openMediaPicker = function(settings = {}) {
    settings = Object.assign({}, defaultSettings, settings);

    const modal = mediaPickerModal(settings);
    document.body.appendChild(modal);
    app.modals.open(modal);
};

function mediaPickerModal(settings) {
    let modal;

    const data = store({
        currentFolderId: "",
        currentFolder: null,
        breadcrumbs: [],
        items: [],
        searchTerm: "",
        selected: [],
        isLoadingItems: false,
        isUploading: false,
        isDragActive: false,
        viewMode: window.localStorage.getItem(viewModeStorageKey) || "list",
        get isLoading() {
            return data.isLoadingItems || data.isUploading;
        },
    });
    let dragDepth = 0;

    const watchers = [
        watch(
            () => settings.selectedPaths,
            () => {
                loadSelected();
            },
        ),
        watch(
            () => [data.currentFolderId, data.searchTerm],
            () => {
                loadCurrentFolder();
                loadItems();
            },
        ),
        watch(
            () => data.viewMode,
            (viewMode) => {
                if (viewMode !== "grid" && viewMode !== "list") {
                    data.viewMode = "list";
                    return;
                }

                window.localStorage.setItem(viewModeStorageKey, viewMode);
            },
        ),
    ];

    function close() {
        setTimeout(() => app.modals.close(modal), 0);
    }

    function loadSelected() {
        data.selected = app.utils.toArray(settings.selectedPaths)
            .map((value) => normalizeMediaFileURL(value))
            .filter(Boolean)
            .map((url) => ({
                url,
                name: getMediaSelectionLabel(url),
            }));
    }

    async function loadCurrentFolder() {
        if (!data.currentFolderId) {
            data.currentFolder = null;
            data.breadcrumbs = [];
            return;
        }

        try {
            const folder = await app.pb
                .collection(mediasCollectionName)
                .getOne(data.currentFolderId, {
                    requestKey: null,
                });

            data.currentFolder = folder;

            const breadcrumbs = [];
            let current = folder;
            while (current) {
                breadcrumbs.unshift(current);

                const parentId = current.parent || "";
                if (!parentId) {
                    break;
                }

                current = await app.pb
                    .collection(mediasCollectionName)
                    .getOne(parentId, {
                        requestKey: null,
                    });
            }

            data.breadcrumbs = breadcrumbs;
        } catch (err) {
            if (err?.status === 404) {
                data.currentFolderId = "";
                data.currentFolder = null;
                data.breadcrumbs = [];
                return;
            }

            app.checkApiError(err);
        }
    }

    async function loadItems() {
        data.isLoadingItems = true;
        app.pb.cancelRequest(listRequestKey);

        try {
            let filter = app.pb.filter("parent={:parent}", {
                parent: data.currentFolderId || "",
            });

            const searchFilter = app.utils.normalizeSearchFilter(data.searchTerm, [
                "name",
            ]);
            if (searchFilter) {
                filter = `(${searchFilter}) && ${filter}`;
            }

            const result = await app.pb
                .collection(mediasCollectionName)
                .getList(1, 200, {
                    requestKey: listRequestKey,
                    filter,
                    sort: "-kind,name",
                });

            data.items = result.items.filter((item) => {
                if (item.kind === "folder") {
                    return true;
                }

                if (!settings.mimeTypes?.length) {
                    return true;
                }

                return settings.mimeTypes.includes(item.mime);
            });
            data.isLoadingItems = false;
        } catch (err) {
            if (!err?.isAbort) {
                data.isLoadingItems = false;
                app.checkApiError(err);
            }
        }
    }

    function openFolder(folder = null) {
        data.currentFolderId = folder?.id || "";
    }

    function setViewMode(mode) {
        data.viewMode = mode === "grid" ? "grid" : "list";
    }

    function toggleSelected(record) {
        if (!record?.id) {
            return;
        }

        const fileURL = getMediaFileURL(record);
        if (!fileURL) {
            openFolder(record);
            return;
        }

        const index = data.selected.findIndex((item) => item.url === fileURL);
        if (index >= 0) {
            data.selected.splice(index, 1);
            return;
        }

        const maxSelect = settings.maxSelect || 1;
        if (maxSelect <= 1) {
            data.selected = [{
                url: fileURL,
                name: record.name,
            }];
            return;
        }

        if (data.selected.length >= maxSelect) {
            data.selected.splice(0, data.selected.length - maxSelect + 1);
        }

        data.selected.push({
            url: fileURL,
            name: record.name,
        });
    }

    function isSelected(record) {
        const fileURL = getMediaFileURL(record);
        if (!fileURL) {
            return false;
        }

        return data.selected.findIndex((item) => item.url === fileURL) >= 0;
    }

    async function submitSelection() {
        await Promise.resolve(settings.onselect(data.selected.map((item) => item.url)));
        app.modals.close(modal);
    }

    async function uploadSingleFile(file, parentId) {
        const formData = new FormData();
        formData.append("kind", "file");
        formData.append("parent", parentId || "");
        formData.append("file", file);

        return app.pb.collection(mediasCollectionName).create(formData);
    }

    async function uploadFiles(fileList, parentId = data.currentFolderId || "") {
        const files = Array.from(fileList || []);
        if (!files.length) {
            return 0;
        }

        let uploaded = 0;

        for (const file of files) {
            if (
                settings.mimeTypes?.length
                && file.type
                && !settings.mimeTypes.includes(file.type)
            ) {
                continue;
            }

            await uploadSingleFile(file, parentId);
            uploaded++;
        }

        return uploaded;
    }

    async function runUpload(uploadAction) {
        if (data.isUploading) {
            return;
        }

        data.isUploading = true;
        app.store.errors = null;

        try {
            const uploaded = await uploadAction();
            if (uploaded > 0) {
                app.toasts.success(
                    `${uploaded} file${uploaded === 1 ? "" : "s"} uploaded.`,
                );
                await loadItems();
            }
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        } finally {
            data.isUploading = false;
        }
    }

    function hasDraggedFiles(event) {
        return Array.from(event.dataTransfer?.types || []).includes("Files");
    }

    function handleDragEnter(event) {
        if (!hasDraggedFiles(event)) {
            return;
        }

        event.preventDefault();
        dragDepth += 1;
        data.isDragActive = true;
    }

    function handleDragOver(event) {
        if (!hasDraggedFiles(event)) {
            return;
        }

        event.preventDefault();
        if (event.dataTransfer) {
            event.dataTransfer.dropEffect = "copy";
        }

        data.isDragActive = true;
    }

    function handleDragLeave(event) {
        if (!hasDraggedFiles(event)) {
            return;
        }

        event.preventDefault();
        dragDepth = Math.max(0, dragDepth - 1);
        if (!dragDepth) {
            data.isDragActive = false;
        }
    }

    async function handleDrop(event) {
        if (!hasDraggedFiles(event)) {
            return;
        }

        event.preventDefault();
        dragDepth = 0;
        data.isDragActive = false;
        await runUpload(() => uploadFiles(event.dataTransfer?.files));
    }

    function rowMeta(record) {
        if (record.kind === "folder") {
            return "Folder";
        }

        const details = [];
        if (record.mime) {
            details.push(record.mime);
        }
        if (record.size) {
            details.push(record.size + " B");
        }

        return details.join(" • ") || "File";
    }

    const documentEvents = {
        "record:create": (e) => {
            if (
                e.detail.collectionName === mediasCollectionName
                || e.detail.collectionId === mediasCollectionName
            ) {
                loadItems();
            }
        },
        "record:update": (e) => {
            if (
                e.detail.collectionName === mediasCollectionName
                || e.detail.collectionId === mediasCollectionName
            ) {
                loadItems();
            }
        },
        "record:delete": (e) => {
            if (
                e.detail.collectionName === mediasCollectionName
                || e.detail.collectionId === mediasCollectionName
            ) {
                loadItems();
            }
        },
    };

    modal = t.div(
        {
            className: "modal popup lg media-picker-modal",
            onafterclose: (el) => {
                el.remove();
            },
            onmount: () => {
                for (const event in documentEvents) {
                    document.addEventListener(event, documentEvents[event]);
                }
            },
            onunmount: () => {
                dragDepth = 0;
                watchers.forEach((w) => w?.unwatch());
                for (const event in documentEvents) {
                    document.removeEventListener(event, documentEvents[event]);
                }
            },
        },
        t.header(
            { className: "modal-header" },
            app.components.searchbar({
                placeholder: "Search media...",
                value: () => data.searchTerm,
                onsubmit: (value) => {
                    data.searchTerm = value;
                },
            }),
            t.div(
                { className: "media-picker-toolbar-actions" },
                t.div(
                    { className: "btn-group media-view-toggle media-picker-view-toggle" },
                    t.button(
                        {
                            type: "button",
                            className: () => `btn sm secondary ${data.viewMode === "list" ? "active" : ""}`,
                            onclick: () => setViewMode("list"),
                        },
                        t.i({ className: "ri-list-check-2", ariaHidden: true }),
                    ),
                    t.button(
                        {
                            type: "button",
                            className: () => `btn sm secondary ${data.viewMode === "grid" ? "active" : ""}`,
                            onclick: () => setViewMode("grid"),
                        },
                        t.i({ className: "ri-grid-line", ariaHidden: true }),
                    ),
                ),
                t.button(
                    {
                        type: "button",
                        className: "btn sm secondary",
                        disabled: () => data.isUploading,
                        onclick: () =>
                            app.modals.openMediaUpload({
                                parentId: data.currentFolderId || "",
                                mimeTypes: settings.mimeTypes || [],
                                oncomplete: async ({ uploaded }) => {
                                    if (uploaded) {
                                        await loadItems();
                                    }
                                },
                            }),
                    },
                    t.i({ className: "ri-upload-cloud-line", ariaHidden: true }),
                    t.span({ className: "txt" }, () => data.isUploading ? "Uploading..." : "Upload"),
                ),
            ),
        ),
        t.div(
            {
                className: () =>
                    `modal-content media-picker-content ${data.isDragActive ? "media-picker-dropzone-active" : ""}`,
                ondragenter: handleDragEnter,
                ondragover: handleDragOver,
                ondragleave: handleDragLeave,
                ondrop: handleDrop,
            },
            t.nav(
                {
                    className: "media-picker-breadcrumbs",
                    ariaLabel: "Current media location",
                },
                t.div(
                    { className: "media-picker-breadcrumbs-trail" },
                    t.button(
                        {
                            type: "button",
                            className: () =>
                                `media-picker-breadcrumb media-picker-breadcrumb-root ${
                                    !data.currentFolderId ? "is-current" : ""
                                }`,
                            onclick: () => openFolder(null),
                        },
                        t.i({ className: "ri-home-5-line", ariaHidden: true }),
                        t.span(
                            { className: "media-picker-breadcrumb-text" },
                            "Root library",
                        ),
                    ),
                    () => {
                        return data.breadcrumbs.flatMap((folder) => {
                            return [
                                t.span(
                                    {
                                        className: "media-picker-breadcrumb-separator",
                                        ariaHidden: true,
                                    },
                                    t.i({ className: "ri-arrow-right-s-line", ariaHidden: true }),
                                ),
                                t.button(
                                    {
                                        type: "button",
                                        className: () =>
                                            `media-picker-breadcrumb ${
                                                folder.id === data.currentFolderId ? "is-current" : ""
                                            }`,
                                        onclick: () => openFolder(folder),
                                    },
                                    t.i({ className: "ri-folder-2-line", ariaHidden: true }),
                                    t.span(
                                        { className: "media-picker-breadcrumb-text" },
                                        folder.name,
                                    ),
                                ),
                            ];
                        });
                    },
                ),
            ),
            t.div(
                {
                    className: () => `list media-picker-list media-picker-list-${data.viewMode}`,
                },
                () => {
                    if (!data.items.length && !data.isLoading) {
                        return t.div(
                            { className: "list-item media-picker-empty" },
                            t.div(
                                { className: "content" },
                                t.span({ className: "txt-hint" }, "No media items found."),
                            ),
                        );
                    }

                    return data.items.map((record) => {
                        const selectable = record.kind === "file" && !!record.file;

                        return t.div(
                            {
                                className: () =>
                                    `list-item highlight media-picker-item media-picker-item-${data.viewMode} ${
                                        isSelected(record) ? "selected" : ""
                                    }`,
                            },
                            t.div(
                                {
                                    className: () =>
                                        `content gap-10 media-picker-item-content media-picker-item-content-${data.viewMode}`,
                                    onclick: () => toggleSelected(record),
                                    ondblclick: async () => {
                                        if (record.kind === "folder") {
                                            openFolder(record);
                                            return;
                                        }

                                        if (!isSelected(record)) {
                                            toggleSelected(record);
                                        }

                                        await submitSelection();
                                    },
                                },
                                () => {
                                    if (record.kind === "file" && record.file) {
                                        return [
                                            app.components.recordFileThumb({
                                                record,
                                                filename: record.file,
                                            }),
                                            t.div(
                                                { className: "media-picker-item-copy" },
                                                t.span({ className: "txt" }, record.name),
                                                t.small({ className: "txt-hint" }, rowMeta(record)),
                                            ),
                                        ];
                                    }

                                    return [
                                        t.span(
                                            { className: "thumb sm media-picker-folder-thumb" },
                                            t.i({ className: "ri-folder-2-line", ariaHidden: true }),
                                        ),
                                        t.div(
                                            { className: "media-picker-item-copy" },
                                            t.span({ className: "txt" }, record.name),
                                            t.small({ className: "txt-hint" }, rowMeta(record)),
                                        ),
                                    ];
                                },
                            ),
                            t.div(
                                { className: "actions" },
                                t.button(
                                    {
                                        type: "button",
                                        className: "btn sm secondary transparent",
                                        hidden: () => record.kind !== "folder",
                                        onclick: () => openFolder(record),
                                    },
                                    t.span({ className: "txt" }, "Open"),
                                ),
                                t.button(
                                    {
                                        type: "button",
                                        className: "btn sm secondary transparent circle",
                                        hidden: () => !selectable,
                                        onclick: () => toggleSelected(record),
                                    },
                                    t.i({
                                        className: () =>
                                            isSelected(record)
                                                ? "ri-checkbox-circle-fill"
                                                : "ri-checkbox-blank-circle-line",
                                        ariaHidden: true,
                                    }),
                                ),
                            ),
                        );
                    });
                },
            ),
            t.div(
                {
                    className: "media-picker-dropzone-overlay",
                    hidden: () => !data.isDragActive,
                },
                t.div(
                    { className: "media-picker-dropzone-panel" },
                    t.i({ className: "ri-upload-cloud-2-line", ariaHidden: true }),
                    t.div({ className: "txt-lg" }, "Drop files to upload"),
                    t.div({ className: "txt-hint" }, () =>
                        data.currentFolder?.name
                            ? `Upload into ${data.currentFolder.name}.`
                            : "Upload into the root media folder."),
                ),
            ),
            t.div(
                {
                    className: "media-picker-selected",
                    hidden: () => !data.selected.length,
                },
                t.div({ className: "txt-hint" }, "Selected"),
                t.div({ className: "media-picker-selected-list" }, () => {
                    return data.selected.map((record) =>
                        t.span(
                            { className: "label" },
                            "FILE",
                            " ",
                            record.name,
                        )
                    );
                }),
            ),
        ),
        t.footer(
            { className: "modal-footer" },
            t.button(
                {
                    type: "button",
                    className: "btn transparent m-r-auto",
                    onclick: close,
                },
                t.span({ className: "txt" }, "Close"),
            ),
            t.button(
                {
                    type: "button",
                    className: "btn",
                    disabled: () => !data.selected.length,
                    onclick: submitSelection,
                },
                t.span({ className: "txt" }, settings.btnText || "Set selection"),
            ),
        ),
    );

    loadSelected();

    return modal;
}
