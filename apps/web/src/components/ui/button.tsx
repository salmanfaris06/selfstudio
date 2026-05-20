import type { ButtonHTMLAttributes } from "react";

export function Button({ className = "", type = "button", ...props }: ButtonHTMLAttributes<HTMLButtonElement>) {
  return <button className={`ui-button ${className}`.trim()} type={type} {...props} />;
}
