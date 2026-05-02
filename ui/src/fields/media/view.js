import { getMediaSelectionLabel, isMediaFileURL } from "@/media/utils";

export function view(props) {
  return t.div(
    {
      className: "record-field-view field-type-media",
    },
    () => {
      const paths = app.utils.toArray(props.record[props.field.name]);
      if (!paths.length) {
        return t.span({ className: "missing-value" });
      }

      const result = [];
      const maxIndex = props.short ? 5 : 100;

      for (let i = 0; i < paths.length; i++) {
        if (i >= maxIndex) {
          result.push(
            t.span({ className: "thumb sm" }, "+", paths.length - maxIndex),
          );
          break;
        }

        const mediaPath = paths[i];
        const isURL = isMediaFileURL(mediaPath);

        if (isURL) {
          const thumb = app.components.fileThumb(mediaPath);

          result.push(
            props.short
              ? thumb
              : t.span(
                  { className: "media-view-item" },
                  thumb,
                  t.span(
                    { className: "label media-path-label" },
                    app.components.copyButton(mediaPath),
                    t.span({ className: "txt txt-ellipsis" }, mediaPath),
                  ),
                ),
          );
          continue;
        }

        result.push(
          props.short
            ? t.span({ className: "label warning", title: mediaPath }, "Legacy")
            : t.span(
                { className: "media-view-item" },
                t.span(
                  { className: "label warning" },
                  getMediaSelectionLabel(mediaPath),
                ),
                t.span(
                  { className: "label media-path-label" },
                  app.components.copyButton(mediaPath),
                  t.span({ className: "txt txt-ellipsis" }, mediaPath),
                ),
              ),
        );
      }

      return result;
    },
  );
}
