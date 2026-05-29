import { Star } from "lucide-react";
import { useEffect, useState } from "react";
import { GITHUB_URL } from "~/lib/config";
import { fetchRepoInfo } from "~/lib/github";
import { GitHubIcon } from "../landing/github-icon";

function formatCount(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
  return String(n);
}

export function DocsHeaderStars() {
  const [stars, setStars] = useState<string | null>(null);

  useEffect(() => {
    fetchRepoInfo().then((data) => {
      if (data) setStars(formatCount(data.stargazers_count));
    });
  }, []);

  return (
    <div className="flex items-center gap-1.5">
      <a
        href={GITHUB_URL}
        target="_blank"
        rel="noreferrer"
        aria-label="GitHub Repository"
        className="inline-flex size-7 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:outline-none"
      >
        <GitHubIcon className="size-3.5" />
      </a>
      {stars && (
        <a
          href={GITHUB_URL}
          target="_blank"
          rel="noreferrer"
          className="inline-flex items-center gap-1 rounded-full border border-border/60 px-2.5 py-1 text-[12px] font-medium text-muted-foreground transition-colors hover:border-border hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:outline-none"
        >
          <Star className="size-3 fill-amber-400 text-amber-400" />
          <span>{stars}</span>
        </a>
      )}
    </div>
  );
}
