#ifndef PRISM_EMBEDDING_H
#define PRISM_EMBEDDING_H

#include <vector>
#include <string>

#ifdef PRISM_USE_ONNX
#include <onnxruntime_cxx_api.h>
#endif

class OnnxEmbedder {
public:
    static OnnxEmbedder& instance();

    // 加载模型，成功返回 true。不调用此函数则使用 TF-IDF
    bool load(const std::string& model_path);
    bool is_loaded() const { return loaded_; }
    int dim() const { return dim_; }

    // 文本向量化，结果长度 = dim()
    std::vector<float> encode(const std::string& text);

private:
    OnnxEmbedder() = default;
    bool loaded_ = false;
    int dim_ = 384;
#ifdef PRISM_USE_ONNX
    std::unique_ptr<Ort::Env> env_;
    std::unique_ptr<Ort::Session> session_;
    std::vector<const char*> input_names_;
    std::vector<const char*> output_names_;
#endif
};

#endif