# Copybara Starlark builtins
# Python stub format
"""Copybara builtins for code transformation workflows."""

from typing import Any, Dict, List, Optional


class Origin:
    """Source repository configuration."""
    ...


class Destination:
    """Target repository configuration."""
    ...


class Transformation:
    """A code transformation operation."""
    ...


class Authoring:
    """Author mapping configuration."""
    ...


class Glob:
    """File pattern matcher."""
    ...


class Integration:
    """Integration configuration for destinations."""
    ...


# core module functions

class core:
    """Core Copybara operations."""

    @staticmethod
    def workflow(
        name: str,
        origin: Origin,
        destination: Destination,
        authoring: Authoring,
        transformations: List[Transformation] = [],
        origin_files: Optional[Glob] = None,
        destination_files: Optional[Glob] = None,
        mode: str = "SQUASH",
    ) -> None:
        """
        Defines a migration workflow from origin to destination.

        This is the main entry point for defining a Copybara workflow.

        Args:
            name: Workflow name
            origin: Source repository
            destination: Target repository
            authoring: Author mapping
            transformations: Transformations to apply
            origin_files: Files to copy from origin
            destination_files: Files to consider in destination
            mode: Workflow mode: SQUASH, ITERATIVE, CHANGE_REQUEST
        """
        ...

    @staticmethod
    def move(
        before: str,
        after: str,
        paths: Optional[Glob] = None,
        overwrite: bool = False,
    ) -> Transformation:
        """
        Move or rename files.

        Args:
            before: Source path pattern
            after: Destination path pattern
            paths: Files to apply to
            overwrite: Overwrite existing files
        """
        ...

    @staticmethod
    def replace(
        before: str,
        after: str,
        regex_groups: Optional[Dict[str, str]] = None,
        paths: Optional[Glob] = None,
        multiline: bool = False,
    ) -> Transformation:
        """
        Replace text content in files.

        Args:
            before: Text to find
            after: Replacement text
            regex_groups: Named regex groups
            paths: Files to apply to
            multiline: Multiline mode
        """
        ...

    @staticmethod
    def transform(
        transformations: List[Transformation],
        ignore_noop: bool = False,
    ) -> Transformation:
        """
        Apply a sequence of transformations.

        Args:
            transformations: Transformations to apply
            ignore_noop: Ignore if no changes
        """
        ...


class git:
    """Git repository operations."""

    @staticmethod
    def origin(
        url: str,
        ref: str = "master",
        submodules: str = "NO",
        include_branch_commit_logs: bool = False,
    ) -> Origin:
        """
        Defines a Git repository as the source of truth.

        Args:
            url: Git repository URL
            ref: Branch, tag, or commit
            submodules: Submodule handling: NO, YES, RECURSIVE
            include_branch_commit_logs: Include branch commit logs
        """
        ...

    @staticmethod
    def github_origin(
        url: str,
        ref: str = "master",
        review_state: Optional[str] = None,
        required_labels: Optional[List[str]] = None,
    ) -> Origin:
        """
        Defines a GitHub repository as origin with PR support.

        Args:
            url: GitHub repository URL
            ref: Branch, tag, or commit
            review_state: PR review state filter
            required_labels: Required PR labels
        """
        ...

    @staticmethod
    def destination(
        url: str,
        push: str,
        fetch: Optional[str] = None,
        integrates: Optional[List[Integration]] = None,
    ) -> Destination:
        """
        Defines a Git repository as the destination.

        Args:
            url: Git repository URL
            push: Branch to push to
            fetch: Branch to fetch from
            integrates: Integrations to apply
        """
        ...

    @staticmethod
    def github_pr_destination(
        url: str,
        destination_ref: str,
        pr_branch: Optional[str] = None,
        title: Optional[str] = None,
        body: Optional[str] = None,
    ) -> Destination:
        """
        Creates GitHub pull requests as destination.

        Args:
            url: GitHub repository URL
            destination_ref: Base branch for PRs
            pr_branch: PR branch name pattern
            title: PR title template
            body: PR body template
        """
        ...


class authoring:
    """Authoring configuration."""

    @staticmethod
    def pass_thru(default: str) -> Authoring:
        """
        Pass through original author information.

        Args:
            default: Default author if not available
        """
        ...

    @staticmethod
    def overwrite(default: str) -> Authoring:
        """
        Overwrite author with a fixed value.

        Args:
            default: Author to use
        """
        ...


def glob(
    include: List[str],
    exclude: List[str] = [],
) -> Glob:
    """
    Create a glob pattern for file matching.

    Args:
        include: Patterns to include
        exclude: Patterns to exclude
    """
    ...
