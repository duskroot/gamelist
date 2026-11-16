import "./styles.css";
import { bootApp } from "./app";

const root = document.querySelector<HTMLElement>("#app");
if (!root) {
  throw new Error("#app missing");
}
bootApp(root);
