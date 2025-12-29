import os
import sys

# Add project root directory to sys.path
project_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "../"))
sys.path.insert(0, project_root)

# Add app directory to sys.path so that modules like 'config', 'logger' can be imported
app_dir = os.path.join(project_root, "app")
sys.path.insert(0, app_dir)

# Add pb folder to sys.path
pb_dir = os.path.join(project_root, "app", "pb")
sys.path.insert(0, pb_dir)
