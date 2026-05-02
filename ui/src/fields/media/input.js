import {
  getMediaSelectionLabel,
  isMediaFileURL,
  normalizeMediaFileURL,
} from "@/media/utils";

export function input(props) {
  const uniqueId = "media_" + app.utils.randomString();

  function triggerChangeEvent() {
    fieldContentEl?.dispatchEvent(
      new CustomEvent("change", {
        detail: { data: props },
        bubbles: true,
      }),
    );
  }

  const local = store({
    get currentPaths() {
      return app.utils
        .toArray(props.record[props.field.name])
        .map((value) => normalizeMediaFileURL(value))
        .filter(Boolean);
    },
    get maxReached() {
      const maxSelect = props.field.maxSelect || 1;
      return local.currentPaths.length >= maxSelect;
    },
  });

  function updateRecordValue(paths = []) {
    const normalized = app.utils
      .toArray(paths)
      .map((value) => normalizeMediaFileURL(value))
      .filter(Boolean);

    props.record[props.field.name] =
      props.field.maxSelect > 1 ? normalized : normalized?.[0] || "";
    triggerChangeEvent();
  }

  function remove(mediaPath) {
    const next = local.currentPaths.filter((path) => path !== mediaPath);
    updateRecordValue(next);
  }

  const fieldContentEl = t.output(
    {
      className: "field-content",
      name: () => props.field.name,
    },
    app.components.sortable({
      className: "list",
      data: () => local.currentPaths,
      onchange: (sortedPaths) => {
        updateRecordValue(sortedPaths);
      },
      dataItem: (mediaPath) => {
        const isURL = isMediaFileURL(mediaPath);

        return t.div(
          {
            rid: mediaPath,
            className: () => `list-item highlight ${isURL ? "" : "warning"}`,
          },
          t.div({ className: "content gap-10" }, () => {
            return [
              isURL
                ? app.components.fileThumb(mediaPath, {
                    width: 100,
                    height: 100,
                  })
                : t.span(
                    { className: "thumb sm" },
                    t.i({
                      className: "ri-error-warning-line",
                      ariaHidden: true,
                    }),
                  ),
              t.div(
                { className: "media-field-item-copy" },
                t.span(
                  { className: "txt txt-break" },
                  getMediaSelectionLabel(mediaPath),
                ),
              ),
            ];
          }),
          t.div(
            { className: "actions" },
            t.button(
              {
                type: "button",
                className: "btn sm secondary transparent circle",
                ariaLabel: app.attrs.tooltip("Remove"),
                onclick: () => remove(mediaPath),
              },
              t.i({ className: "ri-close-line", ariaHidden: true }),
            ),
          ),
        );
      },
    }),
    t.hr({
      className: "m-t-5 m-b-0",
      hidden: () => local.currentPaths.length > 0,
    }),
    t.button(
      {
        type: "button",
        className: "btn sm secondary block",
        onclick: () => {
          app.modals.openMediaPicker({
            selectedPaths: local.currentPaths,
            maxSelect: props.field.maxSelect || 1,
            allowFolders: !!props.field.allowFolders,
            mimeTypes: app.utils.toArray(props.field.mimeTypes),
            onselect: (urls) => {
              updateRecordValue(urls);
            },
          });
        },
      },
      t.i({ className: "ri-folder-open-line", ariaHidden: true }),
      t.span({ className: "txt" }, "Open media picker"),
    ),
  );

  return t.div(
    {
      className: "record-field-input field-type-media",
    },
    t.div(
      {
        className: () => `field-list ${props.field.required ? "required" : ""}`,
      },
      t.label(
        { htmlFor: uniqueId },
        t.i({ className: app.fieldTypes.media.icon, ariaHidden: true }),
        t.span({ className: "txt" }, () => props.field.name),
      ),
      fieldContentEl,
    ),
    () => {
      if (props.field.help) {
        return t.div({ className: "field-help" }, props.field.help);
      }
    },
  );
}
