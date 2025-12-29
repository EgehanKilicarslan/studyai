# 🤝 Contributing Guide

Thank you for considering contributing to this project! To maintain code quality and ensure clear project history, we enforce a **strict commit convention**.

**Please read these rules carefully before submitting code.** Non-compliant commit messages will be **automatically rejected** by our Git hook.

---

## � Commit Message Format

Every commit message **must** follow this format:

```text
<type>(<scope>): <short description>
```

### ✅ Valid Examples

```text
feat(go): add new worker pool implementation
fix(ui): resolve chat bubble overflow issue
chore(ops): update docker-compose networks
docs(docs): improve API documentation
```

### ❌ Invalid Examples

```text
feat: add login                    ❌ Missing scope
update(ui): change colors          ❌ 'update' is not a valid type
fix(backend): fix bug              ❌ 'backend' is not a valid scope (use 'go' or 'py')
Added new feature                  ❌ Doesn't follow format at all
```

---

## 🏷️ Commit Types

The **type** indicates what kind of change you're making:

| Type           | When to Use                                                      | Example                                   |
| -------------- | ---------------------------------------------------------------- | ----------------------------------------- |
| **`feat`**     | Adding a new feature or capability                               | `feat(ui): add dark mode toggle`          |
| **`fix`**      | Fixing a bug or error                                            | `fix(go): prevent memory leak in handler` |
| **`chore`**    | Maintenance tasks (dependencies, configs) that don't affect code | `chore(ops): update nginx to v1.24`       |
| **`refactor`** | Code improvements without changing behavior                      | `refactor(py): simplify embedding logic`  |
| **`docs`**     | Documentation changes only                                       | `docs(docs): update installation steps`   |
| **`style`**    | Code formatting, whitespace (no logic changes)                   | `style(ui): fix indentation in App.tsx`   |
| **`perf`**     | Performance improvements                                         | `perf(go): optimize database queries`     |
| **`test`**     | Adding or fixing tests                                           | `test(py): add unit tests for embeddings` |

---

## 🎯 Valid Scopes

The **scope** indicates which part of the project is affected:

| Scope       | Description                       | Directory/Files                         |
| ----------- | --------------------------------- | --------------------------------------- |
| **`go`**    | Backend Go service (Orchestrator) | `/backend-go/*`                         |
| **`py`**    | Backend Python service (AI Brain) | `/backend-python/*`                     |
| **`ui`**    | Frontend React application        | `/frontend-react/*`                     |
| **`proto`** | gRPC Protobuf definitions         | `/proto/*`                              |
| **`ops`**   | DevOps, Docker, CI/CD             | `Dockerfile`, `docker-compose.yml`, etc |
| **`docs`**  | Documentation files               | `README.md`, `CONTRIBUTING.md`, etc     |

---

## 🚀 Setup Instructions

### 1. Clone the Repository

```bash
git clone https://github.com/EgehanKilicarslan/studyai
cd studyai
```

### 2. Install Git Hooks

Run the setup script to configure commit message validation:

```bash
chmod +x setup-hooks.sh
./setup-hooks.sh
```

**What this does:**

- Configures Git to use hooks from `.githooks/` directory
- Makes the `commit-msg` hook executable
- Validates all future commits against our standards

### 3. Verify Installation

Try making a test commit:

```bash
# This will be rejected
git commit --allow-empty -m "test: invalid commit"

# This will be accepted
git commit --allow-empty -m "test(docs): verify hook installation"
```

---

## 🔍 How the Hook Works

The `.githooks/commit-msg` script automatically:

1. **Extracts** the type and scope from your commit message
2. **Validates** against allowed types and scopes
3. **Rejects** the commit if validation fails
4. **Provides** helpful error messages explaining what's wrong

**Example Error Output:**

```text
❌ Invalid commit type 'update'
   Allowed types: feat, fix, chore, refactor, docs, style, perf, test

❌ Invalid scope 'backend'
   Allowed scopes: go, py, ui, proto, ops, docs
```

---

## � Best Practices

### Write Clear Descriptions

```text
✅ feat(ui): add user profile dropdown menu
❌ feat(ui): added stuff

✅ fix(go): prevent nil pointer dereference in auth middleware
❌ fix(go): fixed bug
```

### Keep Commits Atomic

- One commit = one logical change
- If you're changing multiple scopes, make separate commits

### Use Imperative Mood

```text
✅ add feature
✅ fix bug
✅ update docs

❌ added feature
❌ fixing bug
❌ updated docs
```

---

## 📚 Additional Resources

- [Conventional Commits](https://www.conventionalcommits.org/)
- [How to Write Better Git Commit Messages](https://www.freecodecamp.org/news/how-to-write-better-git-commit-messages/)

---

## ❓ Questions?

If you have questions about commit conventions or need clarification, please open an issue or contact the maintainers.

**Happy coding!** 🎉
