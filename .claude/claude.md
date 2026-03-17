# Rules

## Scoping and orientation
* Always consult `.claude/todo.md to see what the next task is`
* Always read the relevant section of the specification for guidance on the specific task

## Development process
* Always write tests before writing code
* Always perform tasks on a branch named like `<type>/<description>` where type is one of `chore, bug, feature, doc`
* Commits get a logical description line less than 70 chars, followed by a blank line and extra info if the description doesn't give full understanding
* Always run tests after writing code
* Never commit until tests are passing and the user has approved.
* Never merge until the user approves
* Never push until the user approves
* Clean up branches after they are merged
