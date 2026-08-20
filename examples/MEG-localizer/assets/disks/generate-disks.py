import numpy as np
from PIL import Image, ImageDraw

# Canvas & disk parameters.
#
# Matched to the wedge/ring stimuli (generate-wedges-rings.py: SIZE = 800, so
# an outer radius of 400): same canvas, and the orbit set to 0.6 of that outer
# radius so a disk sits in the middle of the eccentricity range the rings
# cover. Keeping the two on one canvas means a disk and a wedge at the same
# polar angle fall on the same part of the visual field.
width, height = 800, 800
center_x, center_y = width / 2, height / 2
orbit_radius = 240  # 0.6 * 400, the wedge/ring outer radius
disk_radius = 30

# Specified polar angles in degrees
angles_deg = [22.5, 67.5, 112.5, 157.5, 202.5, 247.5, 292.5, 337.5]

for i, angle_deg in enumerate(angles_deg):
    # Create white canvas
    img = Image.new("RGB", (width, height), "black")
    draw = ImageDraw.Draw(img)
    
    # Convert angle to radians
    angle_rad = np.radians(angle_deg)
    
    # Calculate disk center (subtracting sin for inverted Y-axis in pixel space)
    x = center_x + orbit_radius * np.cos(angle_rad)
    y = center_y - orbit_radius * np.sin(angle_rad)
    
    # Draw disk
    bbox = [
        x - disk_radius,
        y - disk_radius,
        x + disk_radius,
        y + disk_radius
    ]
    draw.ellipse(bbox, fill="white")
    
    # Save frame
    filename = f"disk_frame_{i+1}.png"
    img.save(filename)
    print(f"Saved: {filename}")
