# Notion tools

A CLI app to perform cleanup in Notion using its REST API.

## Running the App

### Prerequisites
- Receive a Notion [integration token](https://www.notion.so/profile/integrations)

### Installation
Clone the repository and build the app:
```bash
git clone <repository-url>
cd go-notion-tools
go build
```

### Usage
The app extracts values from a specified property in a Notion database.

#### Basic Usage
```bash
./go-notion-tools
```

#### Using Environment Variable
Set the `NOTION_TOKEN` environment variable to avoid passing it as a flag:
```bash
export NOTION_TOKEN=YOUR_NOTION_TOKEN
./go-notion-tools -field "Who"
```

#### Options
- `-unique`: Print unique values only, sorted (default: false)

#### Examples
Extract all values from the "Who" property:
```bash
./go-notion-tools
```

Extract unique values from a custom property:
```bash
./go-notion-tools -unique
```

