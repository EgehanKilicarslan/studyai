# 🎓 StudyAI

**An intelligent document management and RAG (Retrieval-Augmented Generation) system for educational content.**

[![codecov](https://codecov.io/gh/EgehanKilicarslan/studyai/branch/master/graph/badge.svg)](https://codecov.io/gh/EgehanKilicarslan/studyai)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org/)
[![Python](https://img.shields.io/badge/Python-3.11+-3776AB?logo=python)](https://www.python.org/)
[![gRPC](https://img.shields.io/badge/gRPC-Protocol-00ADD8?logo=grpc)](https://grpc.io/)

---

## 📋 Overview

StudyAI is a microservices-based platform that combines document processing, vector search, and large language models to create an intelligent question-answering system. Upload educational materials, ask questions, and receive contextually relevant answers backed by source citations.

**⚠️ Note:** This project is currently under active development (~25% complete). Features and APIs may change significantly.

---

## ✨ Features (Planned & In Progress)

- 🔍 **RAG-Powered Search** - Semantic search with context-aware responses
- 📄 **Multi-Format Support** - PDF, TXT, and Markdown document processing
- 🤖 **LLM Integration** - Support for OpenAI, Anthropic, Google Gemini, and custom models
- 🎯 **Smart Re-ranking** - Improved relevance scoring for search results
- 🔄 **Streaming Responses** - Real-time answer generation via gRPC
- 📊 **Vector Storage** - Efficient embedding management with Qdrant
- 🐳 **Containerized** - Full Docker & Docker Compose support

---

## 🏗️ Architecture

```text
┌─────────────────┐      gRPC       ┌──────────────────┐
│                 │ ◄─────────────► │                  │
│   Backend-Go    │                 │  Backend-Python  │
│ (Orchestrator)  │                 │   (AI Service)   │
│                 │                 │                  │
└────────┬────────┘                 └────────┬─────────┘
         │                                   │
         │                                   │
         ▼                                   ▼
┌─────────────────┐                 ┌──────────────────┐
│                 │                 │                  │
│   PostgreSQL    │                 │   Qdrant DB      │
│  (Metadata)     │                 │  (Embeddings)    │
│                 │                 │                  │
└─────────────────┘                 └──────────────────┘
```

### Components

- **Backend-Go**: Main orchestration service (HTTP/REST API)
- **Backend-Python**: AI/ML service handling embeddings, RAG, and LLM interactions
- **PostgreSQL**: Relational database for document metadata
- **Qdrant**: Vector database for semantic search
- **gRPC**: High-performance inter-service communication

---

## 🚀 Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.21+
- Python 3.11+
- Make

### Installation

```bash
# Clone the repository
git clone https://github.com/EgehanKilicarslan/studyai
cd studyai

# Install dependencies
make install-deps

# Generate protobuf code
make gen-proto

# Start all services
docker-compose up -d
```

### Running Tests

```bash
# Run all tests
make test

# Run specific tests
make test-py  # Python tests
make test-go  # Go tests
```

---

## 🛠️ Configuration

Key environment variables (see `.env.example`):

- `LLM_PROVIDER`: Choose from `openai`, `anthropic`, `gemini`, or `dummy`
- `LLM_API_KEY`: Your LLM provider API key
- `EMBEDDING_MODEL_NAME`: HuggingFace embedding model
- `QDRANT_HOST`: Vector database host
- `POSTGRES_*`: Database connection settings

---

## 📖 Documentation

- Contributing Guide - Learn about commit conventions and development workflow
- API Documentation _(Coming Soon)_
- Architecture Deep Dive _(Coming Soon)_

---

## 🤝 Contributing

We follow strict commit message conventions. Please read CONTRIBUTING.md before submitting any changes.

**Quick Example:**

```bash
git commit -m "feat(py): add reranking service"
git commit -m "fix(go): prevent nil pointer in auth handler"
```

---

## 🧪 Current Status

- [x] Basic RAG pipeline
- [x] Document parsing (PDF, TXT, MD)
- [x] Vector storage integration
- [x] gRPC service definitions
- [ ] Frontend interface (React)
- [ ] User authentication
- [ ] Advanced analytics
- [ ] Production optimizations

---

## 📄 License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

**TL;DR:** You are free to use, modify, and distribute this software, but any derivative work must also be open source under the same license. This ensures the software remains free and open for everyone.

---

## 👨‍💻 Author

**Egehan Kilicarslan**

- GitHub: [@EgehanKilicarslan](https://github.com/EgehanKilicarslan)

---

## ⭐ Acknowledgments

Built with:

- [FastEmbed](https://github.com/qdrant/fastembed) - Fast embedding generation
- [Qdrant](https://qdrant.tech/) - Vector database
- [LangChain](https://www.langchain.com/) - Text processing utilities
- [gRPC](https://grpc.io/) - Inter-service communication

---

**Note:** This is a work-in-progress educational project. Expect frequent updates and breaking changes.
