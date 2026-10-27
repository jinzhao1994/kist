import MapPreview from "./MapPreview";

export default function GPXPreview(props: {
  src: string;
  name: string;
  detail: boolean;
}) {
  return <MapPreview {...props} />;
}
