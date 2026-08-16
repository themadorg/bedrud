export const navLinks = [
  { key: "nav.home", route: "", hash: "", icon: "home" },
  {
    key: "nav.features",
    route: "features",
    hash: "",
    icon: "git-compare",
  },
  {
    key: "nav.screenshots",
    route: "screenshots",
    hash: "",
    icon: "images",
  },
] as const;

export const navRouteLinks = [
  { key: "nav.docs", route: "docs", icon: "book-open" },
  { key: "nav.download", route: "download", icon: "download" },
] as const;
