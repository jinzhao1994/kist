import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { useUploadDropzone } from "./useUploadDropzone";

function transfer() {
  const file = new File(["hello"], "hello.txt", { type: "text/plain" });
  return {
    files: [file],
    items: [{ kind: "file", type: file.type, getAsFile: () => file }],
    types: ["Files"],
  };
}
function Page({
  onDrop,
  disabled = false,
}: {
  onDrop: () => void;
  disabled?: boolean;
}) {
  const drop = useUploadDropzone({ onDrop, disabled, noClick: true });
  return (
    <>
      <aside>Sidebar</aside>
      <main>
        <h1>Box</h1>
        {drop.pasteFiles && (
          <button onClick={drop.confirmPaste}>Confirm paste</button>
        )}
        <section {...drop.getRootProps()} role="region" aria-label="Files">
          <input {...drop.getInputProps()} />
        </section>
      </main>
    </>
  );
}

it("accepts external files on the surrounding page and only once on the list", async () => {
  const onDrop = vi.fn();
  render(<Page onDrop={onDrop} />);
  await act(async () => {
    fireEvent.drop(screen.getByText("Box"), { dataTransfer: transfer() });
  });
  await waitFor(() => expect(onDrop).toHaveBeenCalledTimes(1));
  await act(async () => {
    fireEvent.drop(screen.getByRole("region"), { dataTransfer: transfer() });
  });
  expect(onDrop).toHaveBeenCalledTimes(2);
  await act(async () => {
    fireEvent.drop(screen.getByText("Sidebar"), { dataTransfer: transfer() });
  });
  expect(onDrop).toHaveBeenCalledTimes(2);
});

it("does not automatically upload pasted clipboard images", async () => {
  const onDrop = vi.fn();
  render(<Page onDrop={onDrop} />);
  await act(async () => {
    fireEvent.paste(screen.getByRole("region"), { clipboardData: transfer() });
  });
  expect(onDrop).not.toHaveBeenCalled();
});

it("blocks drops while busy or while a dialog is open", async () => {
  const onDrop = vi.fn();
  const view = render(<Page onDrop={onDrop} disabled />);
  await act(async () => {
    fireEvent.drop(screen.getByText("Box"), { dataTransfer: transfer() });
  });
  expect(onDrop).not.toHaveBeenCalled();
  view.rerender(
    <>
      <Page onDrop={onDrop} />
      <div role="dialog">Editor</div>
    </>,
  );
  await act(async () => {
    fireEvent.drop(screen.getByText("Box"), { dataTransfer: transfer() });
    fireEvent.drop(screen.getByRole("region"), { dataTransfer: transfer() });
  });
  expect(onDrop).not.toHaveBeenCalled();
});

it("passes copied files to intake only after confirmation", async () => {
  const onDrop = vi.fn();
  render(<Page onDrop={onDrop} />);
  const clipboardData = transfer();
  fireEvent.paste(document.body, { clipboardData });
  expect(onDrop).not.toHaveBeenCalled();
  fireEvent.click(await screen.findByRole("button", { name: "Confirm paste" }));
  expect(onDrop).toHaveBeenCalledOnce();
  expect(onDrop.mock.calls[0][0]).toEqual(clipboardData.files);
});
