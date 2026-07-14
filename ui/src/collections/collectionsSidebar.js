const PINNED_STORAGE_KEY = "pbPinnedCollections";
const GROUPS_OPEN_STORAGE_KEY = "pbCollectionGroupsOpen";

const compactThreshold = 12;

export function collectionsSidebar() {
    const data = store({
        search: "",
        pinned: app.utils.getLocalHistory(PINNED_STORAGE_KEY, []),
        openGroups: app.utils.getLocalHistory(GROUPS_OPEN_STORAGE_KEY, {}),
        get filteredCollections() {
            if (!data.search.length) {
                return app.store.collections;
            }

            const normalizedSearch = data.search.replaceAll(" ", "").toLowerCase();

            return app.store.collections.filter((c) => {
                return (c.name + c.id + c.type + (c.collectionGroup || ""))
                    .toLowerCase()
                    .includes(normalizedSearch);
            });
        },
        get systemCollections() {
            return app.utils.sortedCollectionsByType(
                data.filteredCollections.filter(
                    (c) => c.system && !data.pinned.includes(c.id),
                ),
            );
        },
        get regularCollectionSections() {
            return app.utils.partitionCollectionsByGroup(
                data.filteredCollections.filter(
                    (c) => !c.system && !data.pinned.includes(c.id),
                ),
            );
        },
        get pinnedCollections() {
            if (!data.pinned.length) {
                return [];
            }

            return app.utils.sortedCollectionsByType(
                data.filteredCollections.filter((c) => data.pinned.includes(c.id)),
            );
        },
        get totalRegularCollections() {
            return (
                data.regularCollectionSections.ungrouped.length
                + data.regularCollectionSections.groups.reduce(
                    (total, group) => total + group.collections.length,
                    0,
                )
            );
        },
    });

    function clearSearch() {
        data.search = "";
    }

    function isGroupOpen(groupName) {
        const normalized = app.utils.normalizeCollectionGroup(groupName);
        return data.openGroups[normalized] !== false;
    }

    function setGroupOpen(groupName, isOpen) {
        const normalized = app.utils.normalizeCollectionGroup(groupName);
        data.openGroups = {
            ...data.openGroups,
            [normalized]: !!isOpen,
        };
    }

    async function renameGroup(groupName) {
        app.modals.openCollectionGroupUpsert({
            initialName: groupName,
            initialIcon: app.store.getCollectionGroupIcon(groupName),
            title: "Edit collection group",
            submitLabel: "Save",
            onsubmit: async (nextName, icon) => {
                if (nextName === groupName && icon === app.store.getCollectionGroupIcon(groupName)) {
                    return;
                }

                const groups = await app.pb.send(
                    `/api/collections/meta/groups/${encodeURIComponent(groupName)}`,
                    {
                        method: "PATCH",
                        body: { name: nextName, icon: icon || "" },
                    },
                );

                app.store.renameCollectionGroup(groupName, nextName, icon);
                app.store.collectionGroups = app.utils.sortedCollectionGroups(groups || []);

                const normalizedOld = app.utils.normalizeCollectionGroup(groupName);
                if (data.openGroups[normalizedOld] !== undefined) {
                    const nextOpenGroups = { ...data.openGroups };
                    nextOpenGroups[nextName] = nextOpenGroups[normalizedOld];
                    delete nextOpenGroups[normalizedOld];
                    data.openGroups = nextOpenGroups;
                }
            },
        });
    }

    async function deleteGroup(groupName, childCollections, deleteCollections) {
        try {
            await app.pb.send(
                `/api/collections/meta/groups/${encodeURIComponent(groupName)}${
                    deleteCollections ? "?deleteCollections=true" : ""
                }`,
                { method: "DELETE" },
            );

            const childIds = new Set(childCollections.map((collection) => collection.id));
            const activeCollectionId = app.store.activeCollection?.id;
            app.store.removeCollectionGroup(groupName);

            if (deleteCollections) {
                app.store.collections = app.utils.sortedCollectionsByType(
                    app.store.collections.filter((collection) => !childIds.has(collection.id)),
                );
                data.pinned = data.pinned.filter((id) => !childIds.has(id));

                if (childIds.has(activeCollectionId)) {
                    app.store.activeCollection = app.store.collections[0];
                }
            }

            const normalized = app.utils.normalizeCollectionGroup(groupName);
            if (data.openGroups[normalized] !== undefined) {
                const nextOpenGroups = { ...data.openGroups };
                delete nextOpenGroups[normalized];
                data.openGroups = nextOpenGroups;
            }

            if (deleteCollections) {
                app.toasts.success(
                    `Deleted collection group "${groupName}" and ${childCollections.length} ${
                        childCollections.length == 1 ? "collection" : "collections"
                    }.`,
                );
            } else {
                app.toasts.success(`Removed collection group "${groupName}". Its collections are now ungrouped.`);
            }
        } catch (err) {
            app.checkApiError(err);
            return false;
        }
    }

    function removeGroup(groupName) {
        const normalized = app.utils.normalizeCollectionGroup(groupName);
        const childCollections = app.store.collections.filter((collection) => {
            return app.utils.normalizeCollectionGroup(collection.collectionGroup) === normalized;
        });

        if (!childCollections.length) {
            app.modals.confirm(
                `Remove empty collection group "${groupName}"?`,
                () => deleteGroup(groupName, childCollections, false),
                null,
                { yesButton: "Remove group" },
            );
            return;
        }

        app.modals.confirm(
            t.div(
                { className: "block" },
                t.h6({ className: "block txt-center" }, `Remove collection group "${groupName}"?`),
                t.p(
                    { className: "m-t-sm" },
                    `This group contains ${childCollections.length} ${
                        childCollections.length == 1 ? "collection" : "collections"
                    }. Choose whether to delete them and all their records, or keep them as ungrouped collections.`,
                ),
                t.div(
                    { className: "alert warning m-t-sm m-b-0" },
                    "Deleting the collections cannot be undone and may be blocked by references from outside this group.",
                ),
            ),
            () => deleteGroup(groupName, childCollections, true),
            () => deleteGroup(groupName, childCollections, false),
            {
                className: "md",
                yesButton: `Delete all ${childCollections.length}`,
                noButton: "Keep collections",
            },
        );
    }

    const watchers = [];

    return app.components.pageSidebar(
        {
            className: () => `collections-sidebar ${data.responsiveShow ? "active" : ""}`,
            onmount: (el) => {
                // init and persist pinned changes
                watchers.push(
                    watch(() => {
                        app.utils.saveLocalHistory(
                            PINNED_STORAGE_KEY,
                            JSON.stringify(data.pinned),
                        );
                    }),
                );
                watchers.push(
                    watch(() => {
                        app.utils.saveLocalHistory(
                            GROUPS_OPEN_STORAGE_KEY,
                            JSON.stringify(data.openGroups),
                        );
                    }),
                );

                // scroll to the active item
                watchers.push(
                    watch(
                        () => app.store.activeCollection?.id,
                        async () => {
                            await new Promise((r) => setTimeout(r, 0));

                            const activeNavItem = el?.querySelector(".nav-item.active");
                            const details = activeNavItem?.closest("details");
                            if (details) {
                                details.open = true;
                                activeNavItem?.scrollIntoView({ block: "nearest" });
                            }
                        },
                    ),
                );
            },
            onunmount: () => {
                watchers.forEach((w) => w?.unwatch());
            },
        },
        t.div(
            { className: "sidebar-search" },
            t.div(
                { className: "fields" },
                t.div(
                    { className: "field" },
                    t.input({
                        className: "p-r-5",
                        type: "text",
                        placeholder: "Search collections...",
                        value: () => data.search,
                        oninput: (e) => (data.search = e.target.value),
                    }),
                ),
                t.div(
                    { className: "field addon p-l-0 p-r-5 gap-0" },
                    t.button(
                        {
                            hidden: () => !data.search.length,
                            type: "button",
                            className: "btn sm circle transparent secondary",
                            ariaDescription: app.attrs.tooltip("Clear", "left"),
                            onclick: clearSearch,
                        },
                        t.i({ className: "ri-close-line", ariaHidden: true }),
                    ),
                    t.button(
                        {
                            type: "button",
                            disabled: () => app.store.isLoadingCollections,
                            className: () =>
                                `btn sm circle transparent secondary link-faded ${
                                    app.store.isLoadingCollections ? "loading" : ""
                                }`,
                            ariaDescription: app.attrs.tooltip(
                                "Collections overview",
                                "left",
                            ),
                            onclick: () => app.modals.openCollectionsOverview(),
                        },
                        t.i({ className: "ri-organization-chart", ariaHidden: true }),
                    ),
                ),
            ),
        ),
        () => {
            if (
                !data.search.length
                || !!data.filteredCollections.length
                || app.store.isLoadingCollections
            ) {
                return;
            }

            return t.div(
                { className: "block p-t-base txt-center txt-hint" },
                t.p(null, "No collections found."),
                t.button({
                    type: "button",
                    className: "btn sm secondary",
                    textContent: "Clear search",
                    onclick: () => clearSearch(),
                }),
            );
        },
        // show the standalone loader only when there are no other collections loaded
        () => {
            if (app.store.isLoadingCollections && !data.filteredCollections.length) {
                return t.div(
                    { className: "sidebar-content txt-center" },
                    t.span({ className: "loader sm" }),
                );
            }
        },
        () => {
            return [
                t.nav(
                    {
                        className: () =>
                            `sidebar-content collections-list scrollable ${
                                data.totalRegularCollections + data.pinnedCollections.length
                                        >= compactThreshold
                                    ? "compact"
                                    : ""
                            }`,
                    },
                    t.details(
                        {
                            hidden: () => !data.pinnedCollections.length,
                            className: () => `nav-group nav-group-pinned-collections`,
                            open: true,
                        },
                        t.summary(null, "Pinned"),
                        () => data.pinnedCollections.map((c) => collectionItem(c, data)),
                    ),
                    t.details(
                        {
                            hidden: () => !data.regularCollectionSections.ungrouped.length,
                            className: "nav-group nav-group-regular-collections",
                            open: true,
                        },
                        t.summary(null, "Collections"),
                        () => data.regularCollectionSections.ungrouped.map((c) => collectionItem(c, data)),
                    ),
                    () => {
                        return data.regularCollectionSections.groups.map((group) => {
                            return t.details(
                                {
                                    className: "nav-group nav-group-regular-collections",
                                    open: () => isGroupOpen(group.name),
                                    ontoggle: (e) => setGroupOpen(group.name, e.target.open),
                                },
                                t.summary(
                                    {
                                        className: "inline-flex gap-5 flex-nowrap group-summary",
                                    },
                                    t.span(
                                        { className: "collection-group-icon" },
                                        () => {
                                            const icon = app.store.getCollectionGroupIcon(group.name);
                                            if (icon) {
                                                return t.img({
                                                    src: () => app.utils.resolvePublicAssetURL(`icons/${icon}`),
                                                    alt: "",
                                                });
                                            }

                                            return t.i({ className: "ri-folder-line", ariaHidden: true });
                                        },
                                    ),
                                    t.span({ className: "txt" }, group.name),
                                    t.span({ className: "flex-fill" }),
                                    t.span(
                                        { className: "actions" },
                                        // create collection with the group preselected
                                        t.button(
                                            {
                                                type: "button",
                                                className: "btn xs circle transparent secondary",
                                                ariaDescription: app.attrs.tooltip(
                                                    "New collection in group",
                                                    "left",
                                                ),
                                                onclick: (e) => {
                                                    e.preventDefault();
                                                    app.modals.openCollectionUpsert(
                                                        { group: group.name },
                                                        {
                                                            onsave: (newCollection) => {
                                                                app.store.activeCollection = newCollection.id;
                                                            },
                                                        },
                                                    );
                                                },
                                            },
                                            t.i({ className: "ri-add-line", ariaHidden: true }),
                                        ),
                                        t.button(
                                            {
                                                type: "button",
                                                className: "btn xs circle transparent secondary",
                                                ariaDescription: app.attrs.tooltip(
                                                    "Edit group",
                                                    "left",
                                                ),
                                                onclick: async (e) => {
                                                    e.preventDefault();
                                                    e.stopPropagation();
                                                    await renameGroup(group.name);
                                                },
                                            },
                                            t.i({ className: "ri-pencil-line", ariaHidden: true }),
                                        ),
                                        t.button(
                                            {
                                                type: "button",
                                                className: "btn xs circle transparent secondary",
                                                ariaDescription: app.attrs.tooltip(
                                                    "Remove group",
                                                    "left",
                                                ),
                                                onclick: async (e) => {
                                                    e.preventDefault();
                                                    e.stopPropagation();
                                                    await removeGroup(group.name);
                                                },
                                            },
                                            t.i({
                                                className: "ri-delete-bin-line",
                                                ariaHidden: true,
                                            }),
                                        ),
                                    ),
                                ),
                                () => group.collections.map((c) => collectionItem(c, data)),
                            );
                        });
                    },
                    t.details(
                        {
                            hidden: () => !data.systemCollections.length,
                            className: "nav-group nav-group-system-collections",
                            open: () => !!data.search.length,
                        },
                        t.summary(null, "System"),
                        () => data.systemCollections.map((c) => collectionItem(c, data)),
                    ),
                ),
                t.div(
                    {
                        hidden: () => data.search.length && !data.filteredCollections.length,
                        className: "sidebar-content new-collection",
                    },
                    t.div(
                        { className: "collection-create-actions" },
                        t.button(
                            {
                                type: "button",
                                className: "btn outline block",
                                onclick: () => {
                                    app.modals.openCollectionUpsert(
                                        {},
                                        {
                                            onsave: (newCollection) => {
                                                app.store.activeCollection = newCollection.id;
                                            },
                                        },
                                    );
                                },
                            },
                            t.i({ className: "ri-add-line", ariaHidden: true }),
                            t.span({ textContent: "New collection" }),
                        ),
                        t.button(
                            {
                                type: "button",
                                className: "btn secondary block",
                                onclick: () => {
                                    app.modals.openCollectionPresetImport({
                                        onsubmit: (collections) => {
                                            if (collections[0]?.id) {
                                                app.store.activeCollection = collections[0].id;
                                            }
                                        },
                                    });
                                },
                            },
                            t.i({ className: "ri-layout-grid-line", ariaHidden: true }),
                            t.span({ textContent: "From preset" }),
                        ),
                    ),
                ),
            ];
        },
    );
}

function collectionItem(collection, data) {
    return t.button(
        {
            "html-data-collection-id": () => collection.id,
            type: "button",
            className: () =>
                `nav-item responsive-close ${collection.id == app.store.activeCollection?.id ? "active" : ""}`,
            title: () => collection.name,
            onauxclick: (e) => {
                e.preventDefault();
                window.open(
                    `#/collections?collection=${collection.name}`,
                    "_blank",
                    "noreferrer,noopener",
                );
            },
            onclick: (e) => {
                e.preventDefault();
                app.store.activeCollection = collection.name;
            },
        },
        t.i({
            className: () =>
                app.collectionTypes[collection.type]?.icon
                || app.utils.fallbackCollectionIcon,
            ariaHidden: true,
        }),
        t.span({ className: "txt" }, () => collection.name),
        () => {
            if (
                collection.type != "auth"
                || !collection.oauth2?.enabled
                || collection.oauth2?.providers?.length > 0
            ) {
                return;
            }

            return t.i({
                ariaHidden: true,
                className: "ri-alert-line txt-hint txt-sm",
                ariaDescription: app.attrs.tooltip(
                    "OAuth2 auth is enabled but the collection doesn't have any registered providers",
                ),
            });
        },
        () => {
            const pinnedIndex = data.pinned.indexOf(collection.id);

            return t.span(
                {
                    tabIndex: -1,
                    role: "button",
                    className: "pin",
                    title: () => (pinnedIndex >= 0 ? "Unpin" : "Pin"),
                    onclick: (e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        if (pinnedIndex >= 0) {
                            data.pinned.splice(pinnedIndex, 1);
                        } else {
                            data.pinned.push(collection.id);
                        }
                    },
                },
                t.i({
                    ariaHidden: false,
                    className: () => pinnedIndex >= 0 ? "ri-unpin-line" : "ri-pushpin-line",
                }),
            );
        },
    );
}
