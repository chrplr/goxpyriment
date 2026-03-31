import os
import numpy as np
import h5py
import imageio

stimuli_file = "retinotopyStimuli.h5"
output_dir = "Retinotopy/stimuli_png"

if not os.path.exists(output_dir):
    os.makedirs(output_dir)

def save_images(data, folder_name, prefix):
    folder_path = os.path.join(output_dir, folder_name)
    if not os.path.exists(folder_path):
        os.makedirs(folder_path)
    
    print(f"Saving {data.shape[0]} images to {folder_path}...")
    for i in range(data.shape[0]):
        img = data[i].astype(np.uint8)
        fname = os.path.join(folder_path, f"{prefix}_{(i+1):04d}.png")
        imageio.imwrite(fname, img)
        if (i + 1) % 100 == 0:
            print(f"  Saved {i + 1} images")

with h5py.File(stimuli_file, "r") as f:
    patterns = f.get('patterns')
    save_images(patterns, "patterns", "pattern")
    
    masks1 = f.get('masks/expendingCircles')
    save_images(masks1, "masks/expendingCircles", "circle")
    
    masks2 = f.get('masks/rotatingWedge')
    save_images(masks2, "masks/rotatingWedge", "wedge")
    
    masks3 = f.get('masks/swippingBars')
    save_images(masks3, "masks/swippingBars", "bar")
    
    fixationGrid = np.array(f.get('fixationGrid'), dtype=np.uint8)
    imageio.imwrite(os.path.join(output_dir, 'fixationGrid.png'), fixationGrid)

print("Done!")
