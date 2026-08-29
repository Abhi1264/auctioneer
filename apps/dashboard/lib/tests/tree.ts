import type { Check, ViewFilter } from "./types";

export type CheckNode = {
  check: Check;
  children: Check[];
};

export function nestChecks(checks: Check[]): CheckNode[] {
  const roots: Check[] = [];
  const children = new Map<string, Check[]>();

  for (const check of checks) {
    const slash = check.name.indexOf("/");
    if (slash === -1) {
      roots.push(check);
      continue;
    }
    const parent = check.name.slice(0, slash);
    const list = children.get(parent) ?? [];
    list.push(check);
    children.set(parent, list);
  }

  const usedParents = new Set<string>();
  const nodes: CheckNode[] = roots.map((check) => {
    usedParents.add(check.name);
    return { check, children: children.get(check.name) ?? [] };
  });

  for (const [parent, kids] of children) {
    if (!usedParents.has(parent)) {
      nodes.push({ check: kids[0], children: kids.slice(1) });
    }
  }

  return nodes;
}

export function matchesFilter(check: Check, filter: ViewFilter): boolean {
  if (filter === "all") return true;
  return check.status === filter;
}

export function nodeVisible(node: CheckNode, filter: ViewFilter): boolean {
  if (filter === "all") return true;
  if (matchesFilter(node.check, filter)) return true;
  return node.children.some((child) => matchesFilter(child, filter));
}
