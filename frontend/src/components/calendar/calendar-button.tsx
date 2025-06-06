import { useRef } from "react";
import { type AriaButtonProps, useButton, useFocusRing, mergeProps } from "react-aria";
import { Button } from "@/components/ui/button";
import type { CalendarState } from "react-stately";

export function CalendarButton(
  props: AriaButtonProps<"button"> & {
    state?: CalendarState;
    side?: "left" | "right";
    className?: string;
  }
) {
  const ref = useRef<HTMLButtonElement>(null);
  const { buttonProps } = useButton(props, ref);
  const { focusProps } = useFocusRing();
  return (
    <Button
      {...mergeProps(buttonProps, focusProps)}
      ref={ref}
      disabled={props.isDisabled}
      variant="ghost"
      size="icon"
      className={props.className}
    >
      {props.children}
    </Button>
  );
}
