import { getMediaFileURL } from "@/media/utils";

const mediasCollectionName = "_medias";
const folderQueryKey = "folder";
const filterQueryKey = "filter";
const viewModeStorageKey = "pbMediaViewMode";
const perPage = 100;
// PocketBase caps a single list request at 1000 items
const maxPerPage = 1000;

function getStoredViewMode() {
    try {
        return window.localStorage.getItem(viewModeStorageKey) === "grid" ? "grid" : "list";
    } catch {
        return "list";
    }
}

export function pageMedia(route) {
    app.store.title = "Media";
    const uniqueId = "page_media_" + app.utils.randomString();
    // instance scoped so that an unmounting page can't cancel the requests of its replacement
    const listRequestKey = uniqueId + ".list";

    const data = store({
        currentFolderId: route.query[folderQueryKey]?.[0] || "",
        search: route.query[filterQueryKey]?.[0] || "",
        currentFolder: null,
        breadcrumbs: [],
        items: [],
        page: 1,
        totalItems: 0,
        bulkSelected: {},
        isLoading: false,
        isRefreshing: false,
        isLoadingMore: false,
        isUploading: false,
        uploadProgress: { done: 0, total: 0 },
        isDragActive: false,
        isDeleting: {},
        isBulkDeleting: false,
        reset: 0,
        viewMode: getStoredViewMode(),
        get selectedItemsCount() {
            return Object.keys(data.bulkSelected).length;
        },
        get hasMore() {
            return data.items.length < data.totalItems;
        },
        get collection() {
            return (
                app.store.collections.find((c) => c.name === mediasCollectionName)
                || null
            );
        },
    });
    let dragDepth = 0;
    let selectionAnchorId = "";
    // folder+search combination of the currently rendered items
    let loadedContextKey = null;

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

                try {
                    window.localStorage.setItem(viewModeStorageKey, viewMode);
                } catch {}
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
        const contextKey = `${data.currentFolderId}|${data.search}`;
        const isContextChange = contextKey !== loadedContextKey;

        // show the full loader only when switching folder/search,
        // plain refreshes (e.g. realtime events) keep the current items visible
        data.isLoading = isContextChange || !data.items.length;
        data.isRefreshing = true;
        app.pb.cancelRequest(listRequestKey);
        app.pb.cancelRequest(listRequestKey + ".more");

        if (isContextChange) {
            data.page = 1;
        }

        try {
            await Promise.all([loadCurrentFolder(), loadItems()]);
            loadedContextKey = contextKey;
            data.isLoading = false;
            data.isRefreshing = false;
        } catch (err) {
            if (!err?.isAbort) {
                data.isLoading = false;
                data.isRefreshing = false;
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

    function buildItemsFilter() {
        let filter = app.pb.filter("parent={:parent}", {
            parent: data.currentFolderId || "",
        });

        const normalizedSearch = app.utils.normalizeSearchFilter(data.search, [
            "name",
            "mime",
        ]);
        if (normalizedSearch) {
            filter = `(${normalizedSearch}) && ${filter}`;
        }

        return filter;
    }

    async function loadItems() {
        // reload all already loaded pages at once so that a refresh
        // doesn't collapse a previously expanded list
        let loadPerPage = data.page * perPage;
        if (loadPerPage > maxPerPage) {
            data.page = 1;
            loadPerPage = perPage;
        }

        const result = await app.pb
            .collection(mediasCollectionName)
            .getList(1, loadPerPage, {
                requestKey: listRequestKey,
                sort: "-kind,name",
                filter: buildItemsFilter(),
            });

        data.items = result.items;
        data.totalItems = result.totalItems;
        syncSelectionWithItems();
    }

    async function loadMore() {
        if (data.isLoadingMore || data.isLoading || !data.hasMore) {
            return;
        }

        data.isLoadingMore = true;

        try {
            const nextPage = data.page + 1;
            const result = await app.pb
                .collection(mediasCollectionName)
                .getList(nextPage, perPage, {
                    requestKey: listRequestKey + ".more",
                    sort: "-kind,name",
                    filter: buildItemsFilter(),
                });

            data.items = app.utils.filterDuplicatesByKey(data.items.concat(result.items));
            data.totalItems = result.totalItems;
            data.page = nextPage;
            data.isLoadingMore = false;
        } catch (err) {
            if (!err?.isAbort) {
                data.isLoadingMore = false;
                app.checkApiError(err);
            }
        }
    }

    let refreshTimeoutId = null;

    // debounced to collapse bursts of realtime events (bulk uploads/deletes)
    function refresh() {
        clearTimeout(refreshTimeoutId);
        refreshTimeoutId = setTimeout(() => {
            data.reset = Date.now();
        }, 100);
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

    async function copyFileURL(record) {
        const url = getMediaFileURL(record);
        if (!url) {
            return;
        }

        await app.utils.copyToClipboard(new URL(url, window.location.origin).toString());
        app.toasts.success("File URL copied to clipboard.");
    }

    async function downloadFile(record) {
        try {
            const token = await app.getFileToken(record.collectionId);
            const url = app.pb.files.getURL(record, record.file, { token, download: 1 });
            app.utils.download(url, record.name || record.file);
        } catch (err) {
            app.checkApiError(err);
        }
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

    function hasOpenModal() {
        return !!document.querySelector(`[data-modal-state="open"]`);
    }

    function handleDocumentKeydown(event) {
        if (isEditableTarget(event.target) || hasOpenModal()) {
            return;
        }

        if (isSelectAllShortcut(event)) {
            if (!data.items.length) {
                return;
            }

            event.preventDefault();
            selectAllVisible();
            return;
        }

        if (!data.selectedItemsCount) {
            return;
        }

        switch (event.key) {
            case "Escape":
                clearSelection();
                break;
            case "Enter": {
                const selected = Object.values(data.bulkSelected);
                if (selected.length === 1) {
                    event.preventDefault();
                    openItem(selected[0]);
                }
                break;
            }
            case "Delete":
            case "Backspace":
                event.preventDefault();
                confirmBulkDelete();
                break;
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

        await app.pb.collection(mediasCollectionName).create(formData, {
            requestKey: null,
        });

        data.uploadProgress.done++;
    }

    async function uploadFiles(fileList, parentId = data.currentFolderId || "") {
        const files = Array.from(fileList || []);
        if (!files.length) {
            return { files: 0, folders: 0 };
        }

        data.uploadProgress.total += files.length;

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
            data.uploadProgress.total++;
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
            filter: app.pb.filter("parent={:parent}", { parent: parentId }),
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

    function handleItemClick(record, event) {
        if (isDeletingItem(record) || data.isBulkDeleting) {
            return;
        }

        const isMultiSelect = !!(event?.ctrlKey || event?.metaKey);

        // select immediately - a follow-up dblclick opens the item anyway
        if (event?.shiftKey) {
            selectRangeTo(record, isMultiSelect);
        } else if (isMultiSelect) {
            toggleSelected(record);
        } else {
            selectOnly(record);
        }
    }

    // ── Marquee (drag to select) ──
    const marqueeThreshold = 4;
    const marqueeIgnoreSelector = [
        ".media-item",
        "button",
        "a",
        "input",
        "[contenteditable]",
        ".page-header",
        ".media-search-bar",
        ".bulkbar-wrapper",
        ".media-upload-progress",
        ".page-footer",
        ".dropdown",
    ].join(",");
    let marquee = null;

    function toContentPoint(container, clientX, clientY) {
        const rect = container.getBoundingClientRect();
        return {
            x: clientX - rect.left + container.scrollLeft,
            y: clientY - rect.top + container.scrollTop,
        };
    }

    function handleMarqueePointerDown(event) {
        if (
            event.button !== 0
            || event.pointerType === "touch"
            || data.isBulkDeleting
            || !(event.target instanceof Element)
            || event.target.closest(marqueeIgnoreSelector)
        ) {
            return;
        }

        const container = event.currentTarget;

        // ignore presses on the container scrollbars
        const containerRect = container.getBoundingClientRect();
        if (
            event.clientX - containerRect.left >= container.clientWidth
            || event.clientY - containerRect.top >= container.clientHeight
        ) {
            return;
        }

        // prevent native text selection while dragging
        event.preventDefault();
        document.activeElement?.blur?.();

        marquee = {
            container,
            pointerId: event.pointerId,
            start: toContentPoint(container, event.clientX, event.clientY),
            lastClientX: event.clientX,
            lastClientY: event.clientY,
            isActive: false,
            additive: !!(event.ctrlKey || event.metaKey || event.shiftKey),
            baseSelection: { ...data.bulkSelected },
            box: null,
        };

        container.setPointerCapture?.(event.pointerId);
        container.addEventListener("scroll", updateMarquee);
    }

    function handleMarqueePointerMove(event) {
        if (!marquee || event.pointerId !== marquee.pointerId) {
            return;
        }

        marquee.lastClientX = event.clientX;
        marquee.lastClientY = event.clientY;
        updateMarquee();
    }

    function updateMarquee() {
        if (!marquee) {
            return;
        }

        const { container, start } = marquee;
        const current = toContentPoint(container, marquee.lastClientX, marquee.lastClientY);

        if (
            !marquee.isActive
            && Math.abs(current.x - start.x) < marqueeThreshold
            && Math.abs(current.y - start.y) < marqueeThreshold
        ) {
            return;
        }

        if (!marquee.isActive) {
            marquee.isActive = true;
            marquee.box = document.createElement("div");
            marquee.box.className = "media-marquee";
            container.appendChild(marquee.box);
        }

        const left = Math.min(start.x, current.x);
        const top = Math.min(start.y, current.y);
        const width = Math.abs(current.x - start.x);
        const height = Math.abs(current.y - start.y);

        Object.assign(marquee.box.style, {
            left: `${left}px`,
            top: `${top}px`,
            width: `${width}px`,
            height: `${height}px`,
        });

        // compare in viewport coordinates
        const containerRect = container.getBoundingClientRect();
        const box = {
            left: left - container.scrollLeft + containerRect.left,
            top: top - container.scrollTop + containerRect.top,
        };
        box.right = box.left + width;
        box.bottom = box.top + height;

        const itemsById = new Map(data.items.map((item) => [item.id, item]));
        const nextSelected = marquee.additive ? { ...marquee.baseSelection } : {};
        let lastHitId = "";

        for (const el of container.querySelectorAll(".media-item[data-media-id]")) {
            const r = el.getBoundingClientRect();
            const intersects = r.left < box.right
                && r.right > box.left
                && r.top < box.bottom
                && r.bottom > box.top;
            if (!intersects) {
                continue;
            }

            const item = itemsById.get(el.dataset.mediaId);
            if (item) {
                nextSelected[item.id] = item;
                lastHitId = item.id;
            }
        }

        // avoid redundant reactive updates while the pointer moves
        const nextKey = Object.keys(nextSelected).sort().join(",");
        const currentKey = Object.keys(data.bulkSelected).sort().join(",");
        if (nextKey !== currentKey) {
            data.bulkSelected = nextSelected;
        }
        if (lastHitId) {
            selectionAnchorId = lastHitId;
        }
    }

    function endMarquee(event) {
        if (!marquee || (event && event.pointerId !== marquee.pointerId)) {
            return;
        }

        const { container, isActive, additive, box } = marquee;

        container.removeEventListener("scroll", updateMarquee);
        if (container.hasPointerCapture?.(marquee.pointerId)) {
            container.releasePointerCapture(marquee.pointerId);
        }
        box?.remove();
        marquee = null;

        // plain click on an empty area clears the selection
        if (!isActive && !additive && event?.type === "pointerup") {
            clearSelection();
        }
    }

    function handleItemDoubleClick(record) {
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
        data.uploadProgress = { done: 0, total: 0 };
        app.store.errors = null;

        try {
            const counts = await uploadAction();
            const successMessage = buildUploadSuccessMessage(counts || {});

            if (successMessage) {
                app.toasts.success(successMessage);
            }
        } catch (err) {
            if (!err?.isAbort) {
                app.checkApiError(err);
            }
        } finally {
            // refresh also on failure since part of the batch may have been uploaded
            refresh();
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

    function renderItemCard(item) {
        return t.div(
            {
                rid: item.id,
                "html-data-media-id": item.id,
                className: () =>
                    `list-item highlight media-item media-item-${item.kind} media-item-${data.viewMode} ${
                        data.bulkSelected[item.id] ? "media-item-selected" : ""
                    }`,
                onclick: (e) => handleItemClick(item, e),
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
                                    t.i({ className: "ri-folder-open-line", ariaHidden: true }),
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
                                        className: () => `dropdown-item ${isDeletingItem(item) ? "loading" : ""}`,
                                        disabled: () => isDeletingItem(item),
                                        onclick: () => {
                                            confirmDeleteItem(item);
                                            el.hidePopover();
                                        },
                                    },
                                    t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
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
                                    className: "dropdown-item",
                                    onclick: () => {
                                        downloadFile(item);
                                        el.hidePopover();
                                    },
                                },
                                t.i({ className: "ri-download-line", ariaHidden: true }),
                                t.span({ className: "txt" }, "Download"),
                            ),
                            t.button(
                                {
                                    type: "button",
                                    className: "dropdown-item",
                                    onclick: () => {
                                        copyFileURL(item);
                                        el.hidePopover();
                                    },
                                },
                                t.i({ className: "ri-link", ariaHidden: true }),
                                t.span({ className: "txt" }, "Copy URL"),
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
                                    className: () => `dropdown-item ${isDeletingItem(item) ? "loading" : ""}`,
                                    disabled: () => isDeletingItem(item),
                                    onclick: () => {
                                        confirmDeleteItem(item);
                                        el.hidePopover();
                                    },
                                },
                                t.i({ className: "ri-delete-bin-7-line", ariaHidden: true }),
                                t.span({ className: "txt" }, "Delete"),
                            ),
                        ];
                    },
                ),
            ),
            t.div({ className: "content gap-10" }, () => {
                if (item.kind === "folder") {
                    if (data.viewMode === "grid") {
                        return [
                            t.div(
                                { className: "media-folder-thumb-area" },
                                t.i({ className: "ri-folder-open-line", ariaHidden: true }),
                            ),
                            t.div(
                                { className: "media-folder-link" },
                                t.span({ className: "txt" }, item.name),
                            ),
                        ];
                    }
                    return [
                        t.div(
                            { className: "media-folder-thumb-area-list" },
                            t.i({ className: "ri-folder-2-line", ariaHidden: true }),
                        ),
                        t.div(
                            { className: "media-file-meta" },
                            t.div({ className: "txt" }, item.name),
                            t.small({ className: "txt-hint" }, "Directory"),
                        ),
                    ];
                }

                if (data.viewMode === "grid") {
                    return [
                        t.div(
                            { className: "media-thumb-wrapper" },
                            app.components.recordFileThumb({
                                record: item,
                                filename: item.file,
                                extraClasses: "sm",
                            }),
                            t.div(
                                { className: "media-item-badge media-thumb-badge" },
                                getFileTypeBadge(item),
                            ),
                        ),
                        t.div(
                            { className: "media-file-meta" },
                            t.div({ className: "txt" }, item.name),
                            t.small(
                                { className: "txt-hint" },
                                `${app.utils.formattedFileSize(item.size || 0)} \u2022 ${item.mime || "file"}`,
                            ),
                        ),
                    ];
                }

                return [
                    t.div(
                        { className: "media-thumb-wrapper-list" },
                        app.components.recordFileThumb({
                            record: item,
                            filename: item.file,
                            extraClasses: "sm",
                        }),
                    ),
                    t.div(
                        { className: "media-file-meta" },
                        t.div(
                            { className: "media-item-badge" },
                            getFileTypeBadge(item),
                        ),
                        t.div({ className: "txt" }, item.name),
                        t.small(
                            { className: "txt-hint" },
                            `${app.utils.formattedFileSize(item.size || 0)} \u2022 ${item.mime || "file"}`,
                        ),
                    ),
                ];
            }),
        );
    }

    function openUploadModal() {
        app.modals.openMediaUpload({
            parentId: data.currentFolderId || "",
            oncomplete: ({ uploaded }) => {
                if (uploaded) {
                    refresh();
                }
            },
        });
    }

    function renderDropCard() {
        return t.button(
            {
                type: "button",
                className: () => `media-drop-card ${data.viewMode === "list" ? "media-drop-card-list" : ""}`,
                onclick: () => openUploadModal(),
            },
            t.i({ className: "ri-upload-cloud-2-line", ariaHidden: true }),
            t.span(null, "Drop files here"),
        );
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
                app.pb.cancelRequest(listRequestKey + ".more");
                clearTimeout(refreshTimeoutId);
                endMarquee();
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
                onpointerdown: handleMarqueePointerDown,
                onpointermove: handleMarqueePointerMove,
                onpointerup: endMarquee,
                onpointercancel: endMarquee,
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
                        { className: "media-view-toggle" },
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
                        className: () => `btn outline secondary circle rotate-btn media-refresh-btn ${data.isRefreshing && !data.isLoading ? "media-refreshing" : ""}`,
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
                            className: "btn media-btn-upload",
                            disabled: () => data.isBulkDeleting || data.isUploading,
                            onclick: () => openUploadModal(),
                        },
                        t.i({ className: "ri-upload-cloud-line", ariaHidden: true }),
                        t.span({ className: "txt" }, "Upload files"),
                    ),
                ),
            ),
            t.div(
                { className: "media-search-bar" },
                t.i({ className: "ri-search-line media-search-icon", ariaHidden: true }),
                app.components.searchbar({
                    className: "media-searchbar",
                    placeholder: "Search by name or mime type (or a filter, e.g. size>1000000)\u2026",
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
                    if (data.search) {
                        return t.div(
                            { className: "media-empty block txt-center p-base" },
                            t.div({ className: "media-empty-eyebrow" }, "No results"),
                            t.h6({ className: "txt" }, `Nothing matches "${data.search}" in this folder.`),
                            t.button(
                                {
                                    type: "button",
                                    className: "btn sm outline secondary m-t-sm",
                                    onclick: () => (data.search = ""),
                                },
                                t.span({ className: "txt" }, "Clear search"),
                            ),
                        );
                    }

                    return t.div(
                        { className: "media-empty block txt-center p-base" },
                        t.div({ className: "media-empty-eyebrow" }, "No items"),
                        t.h6({ className: "txt" }, "This folder is empty."),
                        t.p(
                            { className: "txt-hint" },
                            "Drop files or folders anywhere on this page, or use the buttons below.",
                        ),
                        t.div(
                            { className: "media-empty-actions" },
                            t.button(
                                {
                                    type: "button",
                                    className: "btn sm outline",
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
                                    className: "btn sm",
                                    onclick: () => openUploadModal(),
                                },
                                t.i({ className: "ri-upload-cloud-line", ariaHidden: true }),
                                t.span({ className: "txt" }, "Upload files"),
                            ),
                        ),
                    );
                }

                const folders = data.items.filter((i) => i.kind === "folder");
                const files = data.items.filter((i) => i.kind !== "folder");
                const sections = [];

                if (folders.length) {
                    sections.push(
                        t.div(
                            { className: "media-section-label" },
                            "Folders",
                        ),
                        t.div(
                            { className: () => `media-browser list media-browser-${data.viewMode}` },
                            ...folders.map((item) => renderItemCard(item)),
                        ),
                    );
                }

                if (files.length) {
                    sections.push(
                        t.div(
                            { className: "media-section-label", style: folders.length ? "margin-top: 20px;" : "" },
                            "Files",
                        ),
                        t.div(
                            { className: () => `media-browser list media-browser-${data.viewMode}` },
                            ...files.map((item) => renderItemCard(item)),
                            renderDropCard(),
                        ),
                    );
                } else {
                    sections.push(
                        t.div(
                            {
                                className: () => `media-browser list media-browser-${data.viewMode}`,
                                style: "margin-top: 12px;",
                            },
                            renderDropCard(),
                        ),
                    );
                }

                if (data.hasMore) {
                    sections.push(
                        t.div(
                            { className: "media-load-more" },
                            t.button(
                                {
                                    type: "button",
                                    className: () => `btn sm outline secondary ${data.isLoadingMore ? "loading" : ""}`,
                                    disabled: () => data.isLoadingMore,
                                    onclick: () => loadMore(),
                                },
                                t.span(
                                    { className: "txt" },
                                    () => `Load more (${data.totalItems - data.items.length} remaining)`,
                                ),
                            ),
                        ),
                    );
                }

                return t.div({ className: "media-sections" }, ...sections);
            },
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
                {
                    className: "media-upload-progress",
                    hidden: () => !data.isUploading,
                    role: "status",
                },
                t.span({ className: "loader sm" }),
                t.span({ className: "txt" }, () =>
                    data.uploadProgress.total
                        ? `Uploading ${Math.min(data.uploadProgress.done + 1, data.uploadProgress.total)} of ${data.uploadProgress.total}\u2026`
                        : "Preparing upload\u2026"),
                t.div(
                    { className: "media-upload-progress-bar" },
                    t.div({
                        className: "media-upload-progress-fill",
                        style: () => `width: ${
                            data.uploadProgress.total
                                ? Math.round((data.uploadProgress.done / data.uploadProgress.total) * 100)
                                : 0
                        }%`,
                    }),
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
                t.span({ className: "txt" }, () =>
                    data.hasMore
                        ? `Items: ${data.items.length} of ${data.totalItems}`
                        : `Items: ${data.totalItems}`),
                app.components.credits(),
            ),
        ),
    );
}
