import { describe, expect, it } from "vitest";
import { buildDomainListQuery, type DomainListFilters } from "@/lib/domains/queries";

const BASE_FILTERS: DomainListFilters = {
  query: "",
  lifecycleStatus: "",
  sourceStatus: "",
  page: 1,
  pageSize: 50,
  sort: "",
  direction: "",
};

describe("buildDomainListQuery (domain query serialization)", () => {
  it("always includes page and page_size", () => {
    const query = buildDomainListQuery(BASE_FILTERS);
    expect(query).toContain("page=1");
    expect(query).toContain("page_size=50");
  });

  it("omits empty optional filters rather than sending blank values", () => {
    const query = buildDomainListQuery(BASE_FILTERS);
    expect(query).not.toContain("query=");
    expect(query).not.toContain("lifecycle_status=");
    expect(query).not.toContain("source_status=");
    expect(query).not.toContain("sort=");
    expect(query).not.toContain("direction=");
  });

  it("includes every populated filter, URL-encoded", () => {
    const query = buildDomainListQuery({
      ...BASE_FILTERS,
      query: "example .com",
      lifecycleStatus: "active",
      sourceStatus: "present",
      page: 3,
      pageSize: 100,
      sort: "expiration",
      direction: "desc",
    });
    const params = new URLSearchParams(query);
    expect(params.get("query")).toBe("example .com");
    expect(params.get("lifecycle_status")).toBe("active");
    expect(params.get("source_status")).toBe("present");
    expect(params.get("page")).toBe("3");
    expect(params.get("page_size")).toBe("100");
    expect(params.get("sort")).toBe("expiration");
    expect(params.get("direction")).toBe("desc");
  });

  it("round-trips through URLSearchParams without corrupting special characters", () => {
    const query = buildDomainListQuery({ ...BASE_FILTERS, query: "a&b=c" });
    const params = new URLSearchParams(query);
    expect(params.get("query")).toBe("a&b=c");
  });
});
