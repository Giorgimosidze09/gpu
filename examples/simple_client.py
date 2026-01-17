#!/usr/bin/env python3
"""
Simple Python client for GPU Orchestration Platform
Example usage of the platform
"""

import requests
import json
import time
import sys

class GPUOrchestratorClient:
    """Client for interacting with GPU Orchestration Platform"""
    
    def __init__(self, api_url="http://localhost:8080"):
        self.api_url = api_url.rstrip('/')
        self.base_url = f"{self.api_url}/v1"
    
    def submit_job(self, name, framework, entrypoint, dataset, gpus, budget, 
                   allow_spot=True, estimated_hours=10.0):
        """Submit a training job to the platform"""
        
        spec_yaml = f"""
job:
  type: training
  framework: {framework}
  entrypoint: {entrypoint}
  resources:
    gpus: {gpus}
    max_gpus_per_node: 8
    requires_multi_node: false
    gpu_memory: 80GB
    cpu_memory: 512GB
  data:
    dataset: {dataset}
    locality: prefer
    replication_policy: none
  constraints:
    budget: {budget}
    allow_spot: {allow_spot}
    min_reliability: 0.9
    performance_weight: 0.0
  execution:
    mode: single_cluster
"""
        
        payload = {
            "name": name,
            "spec_yaml": spec_yaml
        }
        
        response = requests.post(
            f"{self.base_url}/jobs",
            json=payload,
            headers={"Content-Type": "application/json"}
        )
        
        if response.status_code != 201:
            raise Exception(f"Failed to submit job: {response.text}")
        
        return response.json()
    
    def get_job(self, job_id):
        """Get job status and details"""
        response = requests.get(f"{self.base_url}/jobs/{job_id}")
        
        if response.status_code != 200:
            raise Exception(f"Failed to get job: {response.text}")
        
        return response.json()
    
    def wait_for_job(self, job_id, poll_interval=5):
        """Wait for job to complete"""
        print(f"Waiting for job {job_id} to complete...")
        
        while True:
            job = self.get_job(job_id)
            status = job.get("status")
            
            print(f"Status: {status}")
            
            if status in ["completed", "failed", "cancelled"]:
                return job
            
            time.sleep(poll_interval)
    
    def list_jobs(self, status=None):
        """List all jobs"""
        url = f"{self.base_url}/jobs"
        if status:
            url += f"?status={status}"
        
        response = requests.get(url)
        
        if response.status_code != 200:
            raise Exception(f"Failed to list jobs: {response.text}")
        
        return response.json()


def main():
    """Example usage"""
    client = GPUOrchestratorClient()
    
    print("=== GPU Orchestration Platform Client ===\n")
    
    # Submit a job
    print("Submitting training job...")
    job = client.submit_job(
        name="resnet50-imagenet",
        framework="pytorch_ddp",
        entrypoint="s3://my-bucket/train.py",
        dataset="s3://datasets/imagenet",
        gpus=8,
        budget=100,
        allow_spot=True,
        estimated_hours=20.0
    )
    
    job_id = job["id"]
    print(f"Job submitted! ID: {job_id}\n")
    
    # Wait for completion
    final_job = client.wait_for_job(job_id)
    
    print("\n=== Job Complete ===")
    print(f"Status: {final_job['status']}")
    print(f"Cost: ${final_job.get('cost', {}).get('running_usd', 0):.2f}")
    
    if "selected" in final_job:
        selected = final_job["selected"]
        print(f"Provider: {selected.get('provider')}")
        print(f"Region: {selected.get('region')}")
        print(f"Instance: {selected.get('instance_type')}")


if __name__ == "__main__":
    main()
