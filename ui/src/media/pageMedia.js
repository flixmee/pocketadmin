const mediasCollectionName = "_medias";
const folderQueryKey = "folder";
const filterQueryKey = "filter";
const listRequestKey = "pageMedia.list";
const viewModeStorageKey = "pbMediaViewMode";

export function pageMedia(route) {
    app.store.title = "Media";
    const uniqueId = "page_media_" + app.utils.randomString();

    const data = store({
        currentFolderId: route.query[folderQueryKey]?.[0] || "",
        search: route.query[filterQueryKey]?.[0] || "",
        currentFolder: null,
        breadcrumbs: [],
        items: [],
        bulkSelected: {},
        isLoading: false,
        isUploading: false,
        isDragActive: false,
        isDeleting: {},
        isBulkDeleting: false,
        reset: 0,
        viewMode: window.localStorage.getItem(viewModeStorageKey) || "list",
        get selectedItemsCount() {
            return Object.keys(data.bulkSelected).length;
        },
        get collection() {
            return (
                app.store.collections.find((c) => c.name === mediasCollectionName)
                || null
            );
        },
    });
    let dragDepth = 0;
    let selectClickTimeoutId = null;
    let selectionAnchorId = "";

    const watchers = [
        watch(
            () => [data.currentFolderId, data.search, data.reset],
            async () => {
                app.utils.replaceHashQueryParams({
                    [folderQueryKey]: data.currentFolderId || null,
                    [filterQueryKey]: data.search || null,
                });

                await loadMedia();
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

    const documentEvents = {
        "record:create": (e) => {
            if (
                e.detail.collectionName === mediasCollectionName
                || e.detail.collectionId === data.collection?.id
            ) {
                refresh();
            }
        },
        "record:update": (e) => {
            if (
                e.detail.collectionName === mediasCollectionName
                || e.detail.collectionId === data.collection?.id
            ) {
                refresh();
            }
        },
        "record:delete": (e) => {
            if (
                e.detail.collectionName === mediasCollectionName
                || e.detail.collectionId === data.collection?.id
            ) {
                refresh();
            }
        },
    };

    async function loadMedia() {
        data.isLoading = true;
        app.pb.cancelRequest(listRequestKey);

        try {
            await Promise.all([loadCurrentFolder(), loadItems()]);
            data.isLoading = false;
        } catch (err) {
            if (!err?.isAbort) {
                data.isLoading = false;
                app.checkApiError(err);
            }
        }
    }

    async function loadCurrentFolder() {
        if (!data.currentFolderId) {
            data.currentFolder = null;
            data.breadcrumbs = [];
            return;
        }

        let folder;
        try {
            folder = await app.pb
                .collection(mediasCollectionName)
                .getOne(data.currentFolderId, {
                    requestKey: listRequestKey + ".folder",
                });
        } catch (err) {
            if (err?.status === 404) {
                data.currentFolderId = "";
                data.currentFolder = null;
                data.breadcrumbs = [];
                return;
            }
            throw err;
        }

        data.currentFolder = folder;

        const breadcrumbs = [];
        let current = folder;

        while (current) {
            breadcrumbs.unshift(current);

            const parentId = current.parent || "";
            if (!parentId) {
                break;
            }

            current = await app.pb.collection(mediasCollectionName).getOne(parentId, {
                requestKey: listRequestKey + ".breadcrumb." + parentId,
            });
        }

        data.breadcrumbs = breadcrumbs;
    }

    async function loadItems() {
        let filter = `parent='${data.currentFolderId || ""}'`;

        const normalizedSearch = app.utils.normalizeSearchFilter(data.search, [
            "name",
        ]);
        if (normalizedSearch) {
            filter = `(${normalizedSearch}) && ${filter}`;
        }

        const result = await app.pb
            .collection(mediasCollectionName)
            .getList(1, 200, {
                requestKey: listRequestKey,
                sort: "-kind,name",
                filter,
            });

        data.items = result.items;
        syncSelectionWithItems();
    }

    function refresh() {
        data.reset = Date.now();
    }

    function setViewMode(mode) {
        data.viewMode = mode === "grid" ? "grid" : "list";
    }

    function renderBreadcrumbs() {
        const nodes = [
            t.button(
                {
                    type: "button",
                    className: () =>
                        `media-breadcrumb media-breadcrumb-root ${!data.currentFolderId ? "is-current" : ""}`,
                    onclick: () => openFolder(null),
                },
                t.i({ className: "ri-home-5-line", ariaHidden: true }),
                t.span({ className: "media-breadcrumb-text" }, "Root library"),
            ),
        ];

        for (const folder of data.breadcrumbs) {
            nodes.push(
                t.span(
                    {
                        className: "media-breadcrumb-separator",
                        ariaHidden: true,
                    },
                    t.i({ className: "ri-arrow-right-s-line", ariaHidden: true }),
                ),
            );
            nodes.push(
                t.button(
                    {
                        rid: folder.id,
                        type: "button",
                        className: () => `media-breadcrumb ${folder.id === data.currentFolderId ? "is-current" : ""}`,
                        onclick: () => openFolder(folder),
                    },
                    t.span({ className: "media-breadcrumb-text" }, folder.name),
                ),
            );
        }

        return nodes;
    }

    function getFileTypeBadge(record) {
        if (record.kind === "folder") {
            return "DIR";
        }

        const ext = (record.file || "").split(".").pop()?.slice(0, 4) || "FILE";
        return ext.toUpperCase();
    }

    function openFolder(folder) {
        data.currentFolderId = folder?.id || "";
    }

    function previewFile(record) {
        app.modals.openFilePreview(async () => {
            const token = await app.getFileToken(record.collectionId);
            return app.pb.files.getURL(record, record.file, { token });
        });
    }

    function isDeletingItem(record) {
        return !!data.isDeleting[record?.id];
    }

    function syncSelectionWithItems() {
        const allowedIds = new Set(data.items.map((item) => item.id));
        const nextSelected = {};

        for (const [id, item] of Object.entries(data.bulkSelected)) {
            if (allowedIds.has(id)) {
                nextSelected[id] = item;
            }
        }

        data.bulkSelected = nextSelected;

        if (selectionAnchorId && !allowedIds.has(selectionAnchorId)) {
            selectionAnchorId = "";
        }
    }

    function setSelected(record, state) {
        const nextSelected = { ...data.bulkSelected };

        if (state) {
            nextSelected[record.id] = record;
            selectionAnchorId = record.id;
        } else {
            delete nextSelected[record.id];
            if (selectionAnchorId === record.id) {
                selectionAnchorId = Object.keys(nextSelected).at(-1) || "";
            }
        }

        data.bulkSelected = nextSelected;
    }

    function toggleSelected(record) {
        setSelected(record, !data.bulkSelected[record.id]);
    }

    function clearSelection() {
        if (selectClickTimeoutId) {
            clearTimeout(selectClickTimeoutId);
            selectClickTimeoutId = null;
        }
        selectionAnchorId = "";
        data.bulkSelected = {};
    }

    function selectOnly(record) {
        selectionAnchorId = record.id;
        data.bulkSelected = {
            [record.id]: record,
        };
    }

    function selectAllVisible() {
        const nextSelected = {};

        for (const item of data.items) {
            nextSelected[item.id] = item;
        }

        data.bulkSelected = nextSelected;
        selectionAnchorId = data.items.at(-1)?.id || "";
    }

    function selectRangeTo(record, preserveExisting = false) {
        const endIndex = data.items.findIndex((item) => item.id === record.id);
        if (endIndex < 0) {
            return;
        }

        const anchorId = selectionAnchorId || data.items[0]?.id || "";
        const startIndex = data.items.findIndex((item) => item.id === anchorId);

        if (startIndex < 0) {
            selectOnly(record);
            return;
        }

        const rangeStart = Math.min(startIndex, endIndex);
        const rangeEnd = Math.max(startIndex, endIndex);
        const nextSelected = preserveExisting ? { ...data.bulkSelected } : {};

        for (let i = rangeStart; i <= rangeEnd; i++) {
            nextSelected[data.items[i].id] = data.items[i];
        }

        data.bulkSelected = nextSelected;
    }

    function isEditableTarget(target) {
        if (!(target instanceof HTMLElement)) {
            return false;
        }

        return !!target.closest(
            "input, textarea, select, [contenteditable='true'], .cm-editor, .tox",
        );
    }

    function isSelectAllShortcut(event) {
        return !!(
            event.key?.toLowerCase() === "a"
            && (event.ctrlKey || event.metaKey)
        );
    }

    function handleDocumentKeydown(event) {
        if (isSelectAllShortcut(event)) {
            if (isEditableTarget(event.target) || !data.items.length) {
                return;
            }

            event.preventDefault();
            selectAllVisible();
            return;
        }

        if (event.key === "Escape" && data.selectedItemsCount) {
            clearSelection();
        }
    }

    async function createFolderRecord(name, parentId) {
        return app.pb.collection(mediasCollectionName).create({
            name,
            kind: "folder",
            parent: parentId || "",
        });
    }

    async function uploadSingleFile(file, parentId) {
        const formData = new FormData();
        formData.append("kind", "file");
        formData.append("parent", parentId || "");
        formData.append("file", file);

        await app.pb.collection(mediasCollectionName).create(formData);
    }

    async function uploadFiles(fileList, parentId = data.currentFolderId || "") {
        const files = Array.from(fileList || []);
        if (!files.length) {
            return { files: 0, folders: 0 };
        }

        for (const file of files) {
            await uploadSingleFile(file, parentId);
        }

        return {
            files: files.length,
            folders: 0,
        };
    }

    function readEntryFile(entry) {
        return new Promise((resolve, reject) => {
            entry.file(resolve, reject);
        });
    }

    function readDirectoryEntries(reader) {
        return new Promise((resolve, reject) => {
            const entries = [];

            const readBatch = () => {
                reader.readEntries((batch) => {
                    if (!batch.length) {
                        resolve(entries);
                        return;
                    }

                    entries.push(...batch);
                    readBatch();
                }, reject);
            };

            readBatch();
        });
    }

    async function uploadEntry(entry, parentId) {
        if (entry.isFile) {
            const file = await readEntryFile(entry);
            await uploadSingleFile(file, parentId);
            return { files: 1, folders: 0 };
        }

        if (entry.isDirectory) {
            const folder = await createFolderRecord(entry.name, parentId);
            const children = await readDirectoryEntries(entry.createReader());
            const counts = { files: 0, folders: 1 };

            for (const child of children) {
                const childCounts = await uploadEntry(child, folder.id);
                counts.files += childCounts.files;
                counts.folders += childCounts.folders;
            }

            return counts;
        }

        return { files: 0, folders: 0 };
    }

    function getDroppedEntries(dataTransfer) {
        const items = Array.from(dataTransfer?.items || []);
        if (!items.length) {
            return [];
        }

        return items
            .map((item) => item.webkitGetAsEntry?.())
            .filter((entry) => !!entry);
    }

    function hasDraggedFiles(event) {
        return Array.from(event.dataTransfer?.types || []).includes("Files");
    }

    function buildUploadSuccessMessage(counts) {
        const parts = [];

        if (counts.files) {
            parts.push(`${counts.files} file${counts.files === 1 ? "" : "s"}`);
        }

        if (counts.folders) {
            parts.push(`${counts.folders} folder${counts.folders === 1 ? "" : "s"}`);
        }

        return parts.length ? `${parts.join(" and ")} uploaded.` : "";
    }

    async function uploadDropData(dataTransfer) {
        const entries = getDroppedEntries(dataTransfer);

        if (entries.length) {
            const counts = { files: 0, folders: 0 };

            for (const entry of entries) {
                const entryCounts = await uploadEntry(
                    entry,
                    data.currentFolderId || "",
                );
                counts.files += entryCounts.files;
                counts.folders += entryCounts.folders;
            }

            return counts;
        }

        return uploadFiles(dataTransfer?.files, data.currentFolderId || "");
    }

    async function getFolderChildren(parentId) {
        return app.pb.collection(mediasCollectionName).getFullList({
            requestKey: null,
            sort: "-kind,name",
            filter: `parent='${parentId}'`,
        });
    }

    async function deleteMediaRecord(record) {
        if (record.kind === "folder") {
            const children = await getFolderChildren(record.id);
            for (const child of children) {
                await deleteMediaRecord(child);
            }
        }

        await app.pb.collection(mediasCollectionName).delete(record.id);
    }

    async function removeItem(record) {
        if (!record?.id || isDeletingItem(record)) {
            return;
        }

        data.isDeleting[record.id] = true;

        try {
            await deleteMediaRecord(record);
            const nextSelected = { ...data.bulkSelected };
            delete nextSelected[record.id];
            data.bulkSelected = nextSelected;
            app.toasts.success(
                `${record.kind === "folder" ? "Folder" : "File"} deleted.`,
            );
            refresh();
        } catch (err) {
            app.checkApiError(err);
            return false;
        } finally {
            delete data.isDeleting[record.id];
        }
    }

    function confirmDeleteItem(record) {
        const message = record.kind === "folder"
            ? `Delete folder "${record.name}" and all nested items?`
            : `Delete file "${record.name}"?`;

        app.modals.confirm(message, () => removeItem(record), null, {
            yesButton: "Delete",
            noButton: "Cancel",
        });
    }

    async function deleteSelectedItems() {
        const selectedItems = Object.values(data.bulkSelected);
        if (!selectedItems.length || data.isBulkDeleting) {
            return;
        }

        data.isBulkDeleting = true;

        try {
            for (const item of selectedItems) {
                await deleteMediaRecord(item);
            }

            data.bulkSelected = {};
            app.toasts.success(
                `${selectedItems.length} item${selectedItems.length === 1 ? "" : "s"} deleted.`,
            );
            refresh();
        } catch (err) {
            app.checkApiError(err);
            return false;
        } finally {
            data.isBulkDeleting = false;
        }
    }

    function confirmBulkDelete() {
        const count = data.selectedItemsCount;
        if (!count) {
            return;
        }

        app.modals.confirm(
            `Delete ${count} selected item${count === 1 ? "" : "s"}? Nested folder contents will also be removed.`,
            () => deleteSelectedItems(),
            null,
            {
                yesButton: "Delete",
                noButton: "Cancel",
            },
        );
    }

    function openItem(record) {
        if (record.kind === "folder") {
            openFolder(record);
            return;
        }

        previewFile(record);
    }

    function queueItemSelect(record, event) {
        if (isDeletingItem(record) || data.isBulkDeleting) {
            return;
        }

        const isMultiSelect = !!(event?.ctrlKey || event?.metaKey);
        const isRangeSelect = !!event?.shiftKey;

        if (selectClickTimeoutId) {
            clearTimeout(selectClickTimeoutId);
        }

        selectClickTimeoutId = setTimeout(() => {
            if (isRangeSelect) {
                selectRangeTo(record, isMultiSelect);
            } else if (isMultiSelect) {
                toggleSelected(record);
            } else {
                selectOnly(record);
            }
            selectClickTimeoutId = null;
        }, 220);
    }

    function handleItemDoubleClick(record) {
        if (selectClickTimeoutId) {
            clearTimeout(selectClickTimeoutId);
            selectClickTimeoutId = null;
        }

        if (isDeletingItem(record) || data.isBulkDeleting) {
            return;
        }

        openItem(record);
    }

    async function runUpload(uploadAction) {
        if (data.isUploading) {
            return;
        }

        data.isUploading = true;
        app.store.errors = null;

        try {
            const counts = await uploadAction();
            const successMessage = buildUploadSuccessMessage(counts || {});

            if (successMessage) {
                app.toasts.success(successMessage);
                refresh();
            }
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        } finally {
            data.isUploading = false;
        }
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
        await runUpload(() => uploadDropData(event.dataTransfer));
    }

    return t.div(
        {
            pbEvent: "pageMedia",
            className: "page page-media",
            onmount: () => {
                if (!app.store.isLoadingCollections) {
                    loadMedia();
                }

                for (let event in documentEvents) {
                    document.addEventListener(event, documentEvents[event]);
                }
                document.addEventListener("keydown", handleDocumentKeydown);
            },
            onunmount: () => {
                app.pb.cancelRequest(listRequestKey);
                if (selectClickTimeoutId) {
                    clearTimeout(selectClickTimeoutId);
                }
                watchers.forEach((w) => w?.unwatch());
                for (let event in documentEvents) {
                    document.removeEventListener(event, documentEvents[event]);
                }
                document.removeEventListener("keydown", handleDocumentKeydown);
            },
        },
        t.div(
            {
                className: () => `page-content full-height ${data.isDragActive ? "media-dropzone-active" : ""}`,
                ondragenter: handleDragEnter,
                ondragover: handleDragOver,
                ondragleave: handleDragLeave,
                ondrop: handleDrop,
            },
            t.header(
                { className: "page-header compact flex-nowrap media-shell-header" },
                t.nav(
                    {
                        className: "breadcrumbs media-breadcrumbs",
                        ariaLabel: "Current media location",
                    },
                    renderBreadcrumbs,
                ),
                t.div(
                    {
                        className: "page-header-secondary-btns media-header-secondary-btns",
                    },
                    t.div(
                        { className: "btn-group media-view-toggle" },
                        t.button(
                            {
                                type: "button",
                                className: () =>
                                    `btn circle transparent secondary ${data.viewMode === "list" ? "active" : ""}`,
                                ariaLabel: app.attrs.tooltip("List view"),
                                onclick: () => setViewMode("list"),
                            },
                            t.i({ className: "ri-list-check-2", ariaHidden: true }),
                        ),
                        t.button(
                            {
                                type: "button",
                                className: () =>
                                    `btn circle transparent secondary ${data.viewMode === "grid" ? "active" : ""}`,
                                ariaLabel: app.attrs.tooltip("Grid view"),
                                onclick: () => setViewMode("grid"),
                            },
                            t.i({ className: "ri-layout-grid-line", ariaHidden: true }),
                        ),
                    ),
                    app.components.refreshButton({
                        className: "btn outline secondary circle rotate-btn media-refresh-btn",
                        onclick: () => refresh(),
                    }),
                ),
                t.div(
                    {
                        className: "page-header-primary-btns media-header-primary-btns",
                    },
                    t.button(
                        {
                            type: "button",
                            className: "btn outline",
                            disabled: () => data.isBulkDeleting,
                            onclick: () =>
                                app.modals.openMediaFolderUpsert(null, data.currentFolderId, {
                                    onsave: () => refresh(),
                                }),
                        },
                        t.i({ className: "ri-folder-add-line", ariaHidden: true }),
                        t.span({ className: "txt" }, "New folder"),
                    ),
                    t.button(
                        {
                            type: "button",
                            className: "btn",
                            disabled: () => data.isBulkDeleting,
                            onclick: () =>
                                app.modals.openMediaUpload({
                                    parentId: data.currentFolderId || "",
                                    oncomplete: ({ uploaded }) => {
                                        if (uploaded) {
                                            refresh();
                                        }
                                    },
                                }),
                        },
                        t.i({ className: "ri-upload-cloud-line", ariaHidden: true }),
                        t.span({ className: "txt" }, "Upload files"),
                    ),
                ),
            ),
            t.div(
                {},
                app.components.searchbar({
                    className: "media-searchbar",
                    placeholder: "Search media by name...",
                    value: () => data.search,
                    onsubmit: (value) => (data.search = value),
                }),
            ),
            t.div(
                {
                    hidden: () => !app.store.isLoadingCollections || !!data.collection,
                    className: "block txt-center p-base",
                },
                t.span({ className: "loader lg" }),
            ),
            t.div(
                {
                    hidden: () => app.store.isLoadingCollections || !!data.collection,
                    className: "block txt-center p-base",
                },
                t.h6({ className: "txt" }, "Media collection is not loaded yet."),
            ),
            t.div(
                {
                    className: () => `media-browser list media-browser-${data.viewMode}`,
                },
                () => {
                    if (!data.collection) {
                        return;
                    }

                    if (data.isLoading) {
                        return t.div(
                            { className: "block txt-center p-base" },
                            t.span({ className: "loader lg" }),
                        );
                    }

                    if (!data.items.length) {
                        return t.div(
                            { className: "media-empty block txt-center p-base" },
                            t.div({ className: "media-empty-eyebrow" }, "No items"),
                            t.h6({ className: "txt" }, "This folder is empty."),
                            t.p(
                                { className: "txt-hint" },
                                "Upload files or create a new folder to populate this view.",
                            ),
                        );
                    }

                    return data.items.map((item) =>
                        t.div(
                            {
                                rid: item.id,
                                className: () =>
                                    `list-item highlight media-item media-item-${item.kind} media-item-${data.viewMode} ${
                                        data.bulkSelected[item.id] ? "media-item-selected" : ""
                                    }`,
                                onclick: (e) => queueItemSelect(item, e),
                                ondblclick: () => handleItemDoubleClick(item),
                            },
                            t.div(
                                {
                                    className: "media-item-menu",
                                    onclick: (e) => e.stopPropagation(),
                                    ondblclick: (e) => e.stopPropagation(),
                                },
                                t.button(
                                    {
                                        type: "button",
                                        className: "btn sm circle transparent secondary",
                                        title: "More actions",
                                        "html-popovertarget": `${uniqueId}_item_menu_${item.id}`,
                                    },
                                    t.i({ className: "ri-more-2-line", ariaHidden: true }),
                                ),
                                t.div(
                                    {
                                        id: `${uniqueId}_item_menu_${item.id}`,
                                        className: "dropdown left sm",
                                        popover: "auto",
                                    },
                                    (el) => {
                                        if (item.kind === "folder") {
                                            return [
                                                t.button(
                                                    {
                                                        type: "button",
                                                        className: "dropdown-item",
                                                        disabled: () => isDeletingItem(item),
                                                        onclick: () => {
                                                            openFolder(item);
                                                            el.hidePopover();
                                                        },
                                                    },
                                                    t.i({
                                                        className: "ri-folder-open-line",
                                                        ariaHidden: true,
                                                    }),
                                                    t.span({ className: "txt" }, "Open"),
                                                ),
                                                t.button(
                                                    {
                                                        type: "button",
                                                        className: "dropdown-item",
                                                        disabled: () => isDeletingItem(item),
                                                        onclick: () => {
                                                            app.modals.openMediaFolderUpsert(item, "", {
                                                                onsave: () => refresh(),
                                                            });
                                                            el.hidePopover();
                                                        },
                                                    },
                                                    t.i({ className: "ri-edit-line", ariaHidden: true }),
                                                    t.span({ className: "txt" }, "Rename"),
                                                ),
                                                t.button(
                                                    {
                                                        type: "button",
                                                        className: () =>
                                                            `dropdown-item ${isDeletingItem(item) ? "loading" : ""}`,
                                                        disabled: () => isDeletingItem(item),
                                                        onclick: () => {
                                                            confirmDeleteItem(item);
                                                            el.hidePopover();
                                                        },
                                                    },
                                                    t.i({
                                                        className: "ri-delete-bin-7-line",
                                                        ariaHidden: true,
                                                    }),
                                                    t.span({ className: "txt" }, "Delete"),
                                                ),
                                            ];
                                        }

                                        return [
                                            t.button(
                                                {
                                                    type: "button",
                                                    className: "dropdown-item",
                                                    disabled: () => isDeletingItem(item),
                                                    onclick: () => {
                                                        previewFile(item);
                                                        el.hidePopover();
                                                    },
                                                },
                                                t.i({ className: "ri-eye-line", ariaHidden: true }),
                                                t.span({ className: "txt" }, "Preview"),
                                            ),
                                            t.button(
                                                {
                                                    type: "button",
                                                    className: () =>
                                                        `dropdown-item ${isDeletingItem(item) ? "loading" : ""}`,
                                                    disabled: () => isDeletingItem(item),
                                                    onclick: () => {
                                                        confirmDeleteItem(item);
                                                        el.hidePopover();
                                                    },
                                                },
                                                t.i({
                                                    className: "ri-delete-bin-7-line",
                                                    ariaHidden: true,
                                                }),
                                                t.span({ className: "txt" }, "Delete"),
                                            ),
                                        ];
                                    },
                                ),
                            ),
                            t.div({ className: "content gap-10" }, () => {
                                if (item.kind === "folder") {
                                    return [
                                        t.div(
                                            { className: "media-folder-link" },
                                            t.i({
                                                className: () =>
                                                    data.viewMode === "grid"
                                                        ? "ri-folder-open-line"
                                                        : "ri-folder-2-line",
                                                ariaHidden: true,
                                            }),
                                            t.span({ className: "txt" }, item.name),
                                        ),
                                        t.div(
                                            { className: "media-file-meta" },
                                            t.div(
                                                { className: "media-item-badge" },
                                                getFileTypeBadge(item),
                                            ),
                                            t.small({ className: "txt-hint" }, "Directory"),
                                        ),
                                    ];
                                }

                                return [
                                    app.components.recordFileThumb({
                                        record: item,
                                        filename: item.file,
                                        extraClasses: "sm",
                                    }),
                                    t.div(
                                        { className: "media-file-meta" },
                                        t.div(
                                            { className: "media-item-badge" },
                                            getFileTypeBadge(item),
                                        ),
                                        t.div({ className: "txt" }, item.name),
                                        t.small(
                                            { className: "txt-hint" },
                                            `${app.utils.formattedFileSize(item.size || 0)} • ${item.mime || "file"}`,
                                        ),
                                    ),
                                ];
                            }),
                        )
                    );
                },
            ),
            t.div(
                {
                    className: "media-dropzone-overlay",
                    hidden: () => !data.isDragActive,
                },
                t.div(
                    { className: "media-dropzone-panel" },
                    t.i({ className: "ri-upload-cloud-2-line", ariaHidden: true }),
                    t.h5({ className: "txt" }, "Drop files or folders to upload"),
                    t.p({ className: "txt-hint" }, () =>
                        data.currentFolder?.name
                            ? `Upload into ${data.currentFolder.name}.`
                            : "Upload into the root media folder."),
                ),
            ),
            t.div(
                { className: "bulkbar-wrapper" },
                t.div(
                    {
                        hidden: () => !data.selectedItemsCount,
                        className: "bulkbar media-bulkbar",
                    },
                    t.span(
                        { className: "txt" },
                        "Selected ",
                        t.strong(null, () => data.selectedItemsCount),
                        () => ` item${data.selectedItemsCount === 1 ? "" : "s"}`,
                    ),
                    t.button(
                        {
                            type: "button",
                            className: "btn sm secondary pill m-r-auto",
                            onclick: () => clearSelection(),
                        },
                        t.span({ className: "txt" }, "Deselect"),
                    ),
                    t.button(
                        {
                            type: "button",
                            className: () => `btn sm pill outline danger ${data.isBulkDeleting ? "loading" : ""}`,
                            disabled: () => data.isBulkDeleting,
                            onclick: () => confirmBulkDelete(),
                        },
                        t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
                        t.span({ className: "txt" }, "Delete"),
                    ),
                ),
            ),
            t.footer(
                { className: "page-footer" },
                t.span({ className: "txt" }, () => `Items: ${data.items.length}`),
                app.components.credits(),
            ),
        ),
    );
}
