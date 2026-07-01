window.app = window.app || {};
window.app.modals = window.app.modals || {};

let collectionIconMetadataPromise;

const iconFileNameOverrides = {
    I3Dcube: "3dcube.svg",
    I3Square: "3square.svg",
    Profile2User: "profile-2user.svg",
    Receipt21: "receipt-2-1.svg",
    Ui8: "ui8.svg",
};

window.app.modals.openCollectionIconPicker = function(settings = {
    selectedIcon: "",
    onselect: null,
}) {
    const modal = collectionIconPickerModal(settings || {});

    document.body.appendChild(modal);

    app.modals.open(modal);
};

function collectionIconPickerModal(settings) {
    let modal;

    const data = store({
        isLoading: true,
        error: "",
        variants: [],
        categories: [],
        variant: "",
        category: "",
        search: "",
        selectedIcon: settings.selectedIcon || "",
        get activeCategory() {
            return data.categories.find((category) => category.name === data.category) || data.categories[0];
        },
        get filteredIcons() {
            const category = data.activeCategory;
            if (!category) {
                return [];
            }

            const search = data.search.trim().toLowerCase();
            if (!search) {
                return category.icons || [];
            }

            return (category.icons || []).filter((icon) => {
                return `${category.name} ${icon}`.toLowerCase().includes(search);
            });
        },
    });

    function iconFileName(icon) {
        if (iconFileNameOverrides[icon]) {
            return iconFileNameOverrides[icon];
        }

        return String(icon || "")
            .replace(/^I(?=\d)/, "")
            .replace(/(\d)([A-Z][a-z])/g, "$1-$2")
            .replace(/([A-Z])([A-Z][a-z])/g, "$1-$2")
            .replace(/([a-z])([A-Z])/g, "$1-$2")
            .replace(/([a-zA-Z])(\d+)/g, "$1-$2")
            .replace(/[^a-zA-Z0-9]+/g, "-")
            .replace(/^-+|-+$/g, "")
            .toLowerCase() + ".svg";
    }

    function iconPath(icon) {
        if (!data.variant || !data.activeCategory || !icon) {
            return "";
        }

        return `${data.variant}/${data.activeCategory.name}/${iconFileName(icon)}`;
    }

    function publicIconUrl(icon) {
        return app.utils.resolvePublicAssetURL(`icons/${iconPath(icon)}`);
    }

    function parseSelectedIcon() {
        const parts = String(data.selectedIcon || "").split("/");
        if (parts.length < 3) {
            return;
        }

        const [variant, category] = parts;
        if (data.variants.includes(variant)) {
            data.variant = variant;
        }
        if (data.categories.find((item) => item.name === category)) {
            data.category = category;
        }
    }

    async function loadMetadata() {
        data.isLoading = true;
        data.error = "";

        try {
            if (!collectionIconMetadataPromise) {
                collectionIconMetadataPromise = fetch(
                    app.utils.resolvePublicAssetURL("icons/meta-data.json"),
                    { credentials: "same-origin" },
                ).then((response) => {
                    if (!response.ok) {
                        throw new Error("Failed to load icon metadata.");
                    }
                    return response.json();
                });
            }

            const metadata = await collectionIconMetadataPromise;
            data.variants = metadata?.variants || [];
            data.categories = metadata?.categories || [];
            data.variant = data.variants.includes("Linear") ? "Linear" : data.variants[0] || "";
            data.category = data.categories[0]?.name || "";
            parseSelectedIcon();
        } catch (err) {
            data.error = err?.message || "Failed to load icons.";
        } finally {
            data.isLoading = false;
        }
    }

    function applySelection() {
        settings.onselect?.(data.selectedIcon);
        app.modals.close(modal);
    }

    modal = t.div(
        {
            pbEvent: "collectionIconPickerModal",
            className: "modal popup lg collection-icon-picker-modal",
            onmount: () => loadMetadata(),
            onafterclose: (el) => {
                el?.remove();
            },
        },
        t.header(
            { className: "modal-header" },
            t.h5({ className: "m-auto txt-center" }, "Choose icon"),
        ),
        t.div(
            { className: "modal-content" },
            () => {
                if (data.isLoading) {
                    return t.div({ className: "txt-center p-base" }, t.span({ className: "loader" }));
                }

                if (data.error) {
                    return t.div({ className: "alert alert-danger" }, data.error);
                }

                return [
                    t.div(
                        { className: "collection-icon-picker-toolbar" },
                        t.div(
                            { className: "field" },
                            t.label({}, "Variant"),
                            app.components.select({
                                value: () => data.variant,
                                options: () =>
                                    data.variants.map((variant) => ({
                                        value: variant,
                                        label: variant,
                                    })),
                                onchange: (opts) => {
                                    data.variant = opts?.[0]?.value || data.variants[0] || "";
                                },
                            }),
                        ),
                        t.div(
                            { className: "field" },
                            t.label({}, "Category"),
                            app.components.select({
                                value: () => data.category,
                                options: () =>
                                    data.categories.map((category) => ({
                                        value: category.name,
                                        label: category.name,
                                    })),
                                onchange: (opts) => {
                                    data.category = opts?.[0]?.value || data.categories[0]?.name || "";
                                },
                            }),
                        ),
                        t.div(
                            { className: "field collection-icon-search" },
                            t.label({}, "Search"),
                            t.input({
                                type: "text",
                                value: () => data.search,
                                oninput: (e) => (data.search = e.target.value),
                            }),
                        ),
                    ),
                    t.div(
                        { className: "collection-icon-grid" },
                        () => {
                            if (!data.filteredIcons.length) {
                                return t.div({ className: "txt-hint txt-center p-base" }, "No icons found.");
                            }

                            return data.filteredIcons.map((icon) => {
                                const path = iconPath(icon);

                                return t.button(
                                    {
                                        type: "button",
                                        className: () =>
                                            `collection-icon-option ${data.selectedIcon === path ? "active" : ""}`,
                                        title: icon,
                                        onclick: () => {
                                            data.selectedIcon = path;
                                        },
                                    },
                                    t.img({
                                        src: () => publicIconUrl(icon),
                                        alt: "",
                                        loading: "lazy",
                                    }),
                                    t.span({ className: "txt txt-ellipsis" }, icon),
                                );
                            });
                        },
                    ),
                ];
            },
        ),
        t.footer(
            { className: "modal-footer" },
            t.button(
                {
                    type: "button",
                    className: "btn transparent m-r-auto",
                    onclick: () => app.modals.close(modal),
                },
                t.span({ className: "txt" }, "Cancel"),
            ),
            t.button(
                {
                    type: "button",
                    className: "btn",
                    disabled: () => !data.selectedIcon,
                    onclick: applySelection,
                },
                t.span({ className: "txt" }, "Select"),
            ),
        ),
    );

    return modal;
}
