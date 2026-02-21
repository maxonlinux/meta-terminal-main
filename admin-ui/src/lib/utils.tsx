export const safeString = (value: any) => {
  if (value === undefined || value === null) {
    return "--";
  }
  return String(value);
};
